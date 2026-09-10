# AppEditions

Turns raw app screenshots into store-ready App Store and Google Play assets, in
every language a project ships in. Go + [howl-go](../howl-go) + templ +
shadcn-templ, in an OS-native window. No Node anywhere, no Electron, no
canvas in the browser.

Read [`docs/howl-go.llms.txt`](docs/howl-go.llms.txt) before writing framework
code and [`docs/shadcn-templ.md`](docs/shadcn-templ.md) before touching the UI.
Neither set of conventions is guessable.

## The one decision everything rests on

**There is one renderer, and it runs on the server.**

`render.Scene(w, h, screen, settings, copy, sources)` draws a tile. The editor's
preview asks for a 240px tile and gets a PNG; the export asks for 1320×2868 and
gets a PNG. Same function, same call, only `w`/`h` differ.

The app this replaces drew its preview on a `<canvas>` in the page and its
export on the same canvas at full size, which was already the right idea. Moving
it to the server removes the browser from the question entirely: the preview is
not a faithful reimplementation of the export, it *is* the export at a smaller
size. Every tile in the interface is an `<img>` pointed at
`/api/preview/{project}/{screen}`.

Consequences worth knowing before changing anything:

- There is no client-side model of a project. Controls post to an endpoint and
  the page re-renders. `client/public/forge.js` is the whole client, and it
  binds by delegation on `document` because a local navigation replaces
  `#outlet` wholesale.
- Rendering is CPU-bound Go. A preview is ~15 ms, a full-size tile ~300 ms. The
  export fans out one goroutine per tile, bounded to one per core.
- A preview response carries an ETag over everything the drawing depends on, so
  the editor's reload-everything-after-a-change is mostly 304s.

## Build

```bash
make                 # fsapis -> fsroutes -> templ generate -> go build
make run             # the app, in its own window
make dev             # the same, watched: rebuild, restart, reload in place
make serve           # just the server on :9010, for a browser
make dev-web         # the watched loop without a window
make test
make check           # howl check — the conventions, enforced
make package         # dist/AppEditions.app (or the Linux install tree)
make css             # ONLY when you used a Tailwind class no source used before
```

`make dev` is the loop: `howl dev` rebuilds and restarts the server on save
while keeping its proxy port up, and the window attaches to that port once and
reloads its content in place. The window must not be `howl dev`'s child — dev
restarts what it builds, and that would kill and respawn the window on every
save.

`make css` is the only target that touches the stylesheet, and it needs no
Node: Tailwind v4 ships a standalone binary, which the target fetches into
`.howl/` and pins. `client/public/app.css` is a
committed build artifact; run `make css` **before** `make`, never after — the
binary embeds `client/public`, so a stylesheet rebuilt after the go build is one
the server does not serve.

## Layout

```
main.go                     flags, then boot
boot/boot.go                app.New(...) — the same app for the server and the window
desktop/                    the native window; nested module, cgo, WKWebView
internal/model/             the vocabulary: project, screen, settings, template. A leaf.
internal/presets/           the tables: devices, layouts, arrangements, rhythms, sizes, locales
internal/render/            THE renderer: scene, text, frames, shadow, fonts
internal/store/             SQLite documents + the screenshot directory
internal/exporter/          full-size render to disk, and the preview
internal/ai/                claude / codex, for writing and translating copy
server/apis/                typed endpoints — file location is the URL (fsapis)
server/handlers/            the page loader, the PNG renderer, the multipart upload
client/pages/               the routes (fsroutes)
client/ui/                  shared components + the icon registry
client/view/                what a page is allowed to see
client/styles/app.css       stylesheet source -> client/public/app.css
```

## Invariants

Each of these exists because breaking it produced a real bug.

1. **Resolve overrides in exactly one place** — the top of `render.Scene`.
   Callers pass the raw screen and the project settings. Two places resolving
   inheritance would eventually disagree, and the disagreement would be
   invisible until an exported PNG came out different from the preview it was
   approved from.

2. **An override of `nil` means inherit.** Never write a resolved value into
   `Overrides` to mean "same as the set" — that silently pins it and the screen
   stops following the global for the rest of its life. `apistore.Setting.Pin`
   is the only thing that writes one.

3. **A `font.Face` is not safe for concurrent use.** `opentype.Face` keeps one
   glyph buffer and one rasteriser. Two goroutines drawing through the same face
   panic inside `image/draw` with an index out of range — and only once the
   export renders tiles in parallel. `render.Face` hands out faces behind a
   mutex; keep it that way.

4. **Fonts are static TTFs, not variable.** `x/image/font/sfnt` reads a variable
   font's default instance and cannot select an axis, so a variable Inter would
   draw every headline at weight 400 however loudly the renderer asked for 700.

5. **Non-Latin text needs the fallback chain.** No Latin face has kana; a
   Japanese headline through the primary face alone renders as a row of nothing.
   `render.Face` builds the chain, and CJK is tokenised per character because it
   is written without spaces and would otherwise never break a line.

6. **Two struct fields tagged `id` at the same depth drop both.** The stored
   types embed `db.Doc` next to a domain type; `model.Project.ID` and
   `model.Template.ID` are `json:"-"` and filled from the envelope on the way
   out. This is not an error in Go — the document just comes back with no
   identifier, and everything downstream 404s.

7. **Pages may not import `core/app`, `db`, or `internal/store`.** The generated
   route table lives inside the page tree. Pages read `client/view`, which
   `server/handlers.Loader` fills. `howl check` enforces the first two.

8. **Route-dependent markup does not go in `client/pages/app.templ`.** The shell
   renders once on a cold load; a local navigation swaps `#outlet` and nothing
   else. The navigation lives in `layout.templ` for that reason.

   The other half of that trade: everything in the outlet is *rebuilt* on every
   navigation, chrome included, and a rebuilt dark panel flashes white for a
   frame. `[data-chrome-backdrop]` is the shell's answer — it holds the top
   bar's and the sidebar's colour still, from outside the outlet, and works out
   what to draw from the sidebar's own attributes rather than from the route.

9. **A closed panel is `display:none`, and `display:none` markup is still
   there.** Anything expensive inside a shut disclosure must be gated on whether
   it is open, or it runs forever for nobody.

10. **The export replaces its language directory.** A screen deleted between
    exports must not leave its file behind, or the upload picks up a screenshot
    the set no longer contains.

11. **One screen is one store tile and one uploaded file.** A panorama is one
    drawing across two tiles, so it is *two screens*: the lead, and a screen
    carrying `Of`/`Part` that draws the other slice. Each renders the whole
    composition and keeps its own column, cut where the export cuts. That is
    what lets either half be moved, reordered or deleted like any other
    screenshot — including into an order that breaks the picture, which is the
    author's call. Never show the un-sliced composition: it is near-square,
    sits wrong beside the portrait tiles, and hides where the cut falls.

    The parts are reconciled by `model.SyncParts`, bound to the layout table as
    `presets.Sync`, and it runs in exactly two places — `client/store.apply`
    and `store.Edit` — so a set never holds a panorama with one half missing.
    Follower ids are derived from the lead's (`<lead>-2`), because `Edit` may
    run its closure again under the optimistic lock and a minted id would
    append a second half instead of rewriting the same one. A composition is
    edited in one place: the words, the screenshot and the pinned settings all
    live on the lead, and `Project.Lead` is what every write resolves through.

12. **A disclosure's sibling selector must be `+`, never `~`.** `~` matches
    every following sibling, so a page with one modal per row has each
    unchecked checkbox holding shut every modal after it, and only the first in
    the document opens. This shipped once as "new project and delete do
    nothing".

13. **A modal inside a transformed element is trapped in it.** `position:
    fixed` resolves against the nearest ancestor carrying a transform. The
    project cards hover-translate, so their edit and delete modals are rendered
    after the list rather than inside the card that opens them.

14. **`card.Card` has no horizontal padding.** `cn-card` sets `padding-block`
    only; `card.Content` is what supplies `px-6`. A form dropped straight into a
    card touches both borders.

15. **A `<button>` inside a `<label for="…">` never activates the label.** A
    label does not forward activation to an interactive element, so the panel
    never opens and nothing says why. Panels are toggled with
    `ui.PanelToggle(id)` and file dialogs with `ui.Picks(id)`, both handled by
    one delegated listener in `forge.js`.

## The window

`desktop/` is a nested cgo module: the same application, in a WKWebView.

**The webview binding leaves the WKWebView with no UI delegate.** Its vendored
`webview.h` autoreleases the delegate it creates and assigns it to
`WKWebView.UIDelegate`, which is a *weak* property — so the pool drains, the
object is deallocated, and the property goes nil. A WKWebView with no UI
delegate silently ignores `<input type="file">`: no panel, no error, no console
message. It works in a browser and is inert in the window.

`desktop/filepanel_darwin.m` installs one that is strongly held, and
`installFilePanel` is called before `desktop.Run`. Verified by reading
`UIDelegate` back: `(nil)` before, `AppKitPanelDelegate` after.

`howl dev` restarts the *server*, not the window — so a change to anything
under `desktop/` needs `make dev` restarted, not just saved.

**The menu bar comes from the framework, not from here.** `desktop.Run`
installs an app/Edit/Window menu because on macOS ⌘C, ⌘V, ⌘Q and ⌘W are menu
*item* behaviour: with no `NSMenu` they do nothing anywhere in the page, which
presents as "the text fields are broken". Do not add a second one.

### Shipping it

```bash
make package                 # dist/AppEditions.app
make package VERSION=1.1.0   # what Get Info shows
```

`make package` does not build — `make desktop` does. It writes the metadata the
OS needs before it will show an icon instead of a blank sheet of paper: a
`.app` on macOS, and on Linux a `.desktop` entry, the hicolor PNGs and an
`install.sh`. Everything is derived from **`desktop/packaging/icon.png`**, one
square 1024px PNG, in pure Go — so there is no `iconutil`/`sips` step and
nothing to install.

`APP_ID` in the Makefile is what Launch Services keys its icon and permission
caches on. It must not change once a build has been installed anywhere, or the
old identifier leaves a ghost still claiming the old icon.

`TARGET=linux`, never `GOOS=linux`: make exports a command-line variable into
every recipe, so `GOOS` cross-compiles `fsapis` and the build dies trying to run
it. The desktop binary needs cgo and the platform's webview and cannot be
cross-compiled at all — build and package the Linux one on Linux. `howl
package` refuses the mismatch rather than producing a tree that installs,
appears in the launcher and does nothing when clicked.

To add `LSMinimumSystemVersion`, a URL scheme or a document type, put a real
`Info.plist` in `desktop/packaging/` — it is copied verbatim. Same for
`app.desktop` on Linux. There are no flags for these on purpose.

## The store: one implementation of the rules, two places

`client/store/` is the editor's state and the only copy of the rules that
change it. It is howl-go's third mechanism, and it is here for the reason the
framework names — the *same rules* have to run on the server and in the
browser, and there must not be two implementations of them.

```
client/store/editor.go         domain — compiles for the server AND GOOS=js
client/store/editor_client.go  signals — package-level, therefore browser-only
```

`editor.go` holds `model.Project`, the `Op` vocabulary, `Apply`/`ApplyTo`, and
the derived reads (`Spans`, `Readiness`). Nothing server-shaped may enter it —
no `net/http`, no `database/sql`, no `os` — because the pages import it and the
pages compile to wasm. Verify with `GOOS=js GOARCH=wasm go build ./client/store/`.

The three wires:

1. **SSR** — the loader puts the document on the context; the page renders from
   it, with no JavaScript at all.
2. **Hydrate** — `GET /api/projects/{id}/snapshot` returns `EditorSnapshot`,
   which is the project document itself rather than a projection, so there is
   nothing to keep in step as the model grows.
3. **Local** — `EditorClient().Apply(op)` mutates, `publish()` sets the signal,
   and the same `Op` is posted to `/api/projects/{id}/ops`, where the server
   runs the same `ApplyTo`.

`client/store/editor_test.go` asserts wire 3's claim directly: every op is
applied through the store and through `ApplyTo`, and the two documents must
encode identically. If that test fails, an optimistic update has started lying
about what was saved.

Two consequences worth knowing. `presets.Span`/`ShowsText` live in the preset
tables rather than the renderer, because the browser needs the answers and the
renderer embeds twelve megabytes of fonts. `model.Setting` — the control-value
language — lives in the vocabulary rather than the endpoint layer, because both
sides parse the same `{key, value}` and "a tilt is a float" has to mean the same
thing in both.

**The server must never write a signal.** They are package-level; a request
handler setting one is two requests writing one variable. `howl check` reports
server code importing `core/signal`.

## Two client rules that fail silently

**Links are document loads, not fragment swaps.** `howl.navigate` replaces
`#outlet` — which is where the top bar and the sidebar live, so they are torn
down and rebuilt on every step change — and it may serve the fragment from a
cache filled before the click. Both of those buy something across a network and
nothing at all here, where the server is in this process: a swap flashed the
chrome, and a cached fragment described a set two edits ago or a sidebar that
had since been collapsed.

So `forge.js` intercepts link clicks in the capture phase and sets
`location.href`. Capture is what makes it possible — howl's own handler is on
`document` in the bubble phase and skips an event whose default is already
prevented — and the eligibility test there mirrors the framework's `spaTarget`,
which it has to: a link it takes over but howl would have declined simply stops
working. `data-no-spa` says the same thing declaratively but is read off the
anchor, not an ancestor, so it cannot be set once; `<body data-no-prefetch>`
*is* read off the nearest ancestor and stops the hover fetch.

**The one thing that still swaps is the re-render after a write.** `refresh()`
passes `fresh: true`, so its fragment is never the cached one, and reloading
the document on every nudge of a slider would throw away the scroll position
and the control's focus. That swap is why `[data-chrome-backdrop]` exists.

**Preview tiles render at 2× their layout width.** The `width` attribute is the
CSS size and the pixels behind it are double, because every display this runs
on is Retina and a tile rendered at its CSS width is upscaled by the browser.
`store.previewWidth` has to stay above the largest frame the editor draws at
2× or the source is upscaled inside a sharp render, which looks worse than
either.

## Verifying a change

The preview is a rendered PNG. The DOM says nothing about whether the drawing is
right — an accessibility tree cannot see a wrong bezel, a clipped headline or a
font that silently fell back.

- `go test ./internal/render/` writes one PNG per look into
  `$TMPDIR/appeditions-render`. Look at them.
- Pixel claims go through the export, not the preview: render at store size and
  check the dimensions and the bytes.
- `make check` catches the framework mistakes that fail silently.

## Targets and languages

A project ships to a list of **targets** and a list of **languages**, and the
export is the product of the two: one directory per pair, which is one upload.

A target is a store slot — the pixel size the store asks for, and the device
frame drawn in it. `presets.Sizes` carries the frame that belongs in each slot,
so ticking "iPad 13-inch" draws an iPad body rather than inheriting whatever
the last size used.

`Settings.SizeID`/`DeviceID` are the target *currently being drawn*.
`Project.Use(target)` resolves one into them; the editor does it from `?size=`
and the export does it once per target. Nothing else in the renderer knows
targets exist, which is why adding them changed no drawing code.

The **screenshots are per-language too**, but only where they need to be.
`Screen.AssetID` is the base language's capture and every language draws it;
`Screen.Shots[locale]` replaces it for one language, because a screenshot is a
picture of the app and a Japanese listing showing an English UI is not the app
anyone is downloading. Absent means inherit, exactly as a nil override does —
so clearing under a translation goes back to the base picture rather than
emptying the slot, and a project that ships one set of captures never writes
the field at all. `Screen.Asset(locale)` resolves it, in one place, and
`PreviewTag` hashes what that returns: a tag that hashed `AssetID` would leave
the German tile revalidating to a 304 and drawing the English picture forever.

## Data

Everything lives under `~/.appeditions` (or `$APPEDITIONS_HOME`):

```
appeditions.db              projects, templates, assets — SQLite, one JSON document per row
<project-slug>/
  assets/<id>.png       the originals, never modified
  exports/<size>/<locale>/01-….png
  exports/<size>/<locale>/listing.txt   the store text for that language
templates/<template-id>/01.png   the captures a saved look was designed against
fonts/                  optional extra fonts, tried before the system's
```

A saved template keeps a *copy* of the screenshots it was designed against, in
its own directory beside the projects rather than inside one. A look is not
separable from the pictures it was judged on — a headline sits where it does
because of what was behind it — and the project a template was saved from is
the one most likely to be deleted, because the template is what replaced it.
`store.importShots` files them into the target as ordinary assets on apply, so
they are replaceable and listed in Media like anything dropped in by hand, and
only for the slots the project has no picture of its own for.

`boot.Commit` is the short sha the binary was built from, stamped by the
Makefile. It is the version that matters: there are no releases and no tags,
`install.sh` builds whatever `main` points at, and running it again is how you
update — so being out of date is that commit no longer being main's head.
`internal/update` asks GitHub once every six hours in its own goroutine, never
in the path of a render, and the sidebar grows an "Update available" button
only when the answer is yes. Every failure is silent, and a build with nothing
stamped in it (a `go run`, a source tarball) does not ask at all.

The button runs **`update.sh`**, the project's own updater, fetched and piped to
`sh` the way its own documentation says to — an install keeps only
`uninstall.sh`, so there is no local copy to run. The script is the whole
implementation and none of it is reimplemented in Go: it compares the revision
`install.sh` recorded at `~/.local/state/appeditions/source-revision` against
the tip, and rebuilds only if they differ. Note that this is a *different*
question from the one the badge asks — the receipt says what was last
installed, `boot.Commit` says what is running — and they disagree for exactly
as long as it takes to relaunch.

The request is held open for the whole build, about a minute, because the
alternative is a job queue and a page that polls it and this application has
neither. What comes back is the script's own last line. The new build is on
disk when it does; the process serving the request is still the old one, so the
message says to quit and reopen.

A project is one document: the look, the languages, and a list of **versions**.
An edit is one patch and one revision.

A version is one submission — `1.0.1`, free text, 32 characters — and it owns
its own screens, its own store slots and its own store listing. A release can
ship to fewer devices than the last one did; the export writes only the
directories that version names. That is what a version *is*:
shipping 1.1 means new captures and a new "What's New" beside the old ones, and
an author who wants last release's set back wants it exactly as it was. The
*look* stays on the project, because a house style is the thing that does not
change between releases.

Everything the editor shows hangs off `Project.VersionID`, resolved in exactly
one place — `Project.version()` for writes, `current()` for reads. `p.Screens()`
and `p.Targets()` are methods, not fields, for that reason.

**`?version=` is persisted by the loader, not merely applied.** The size and the
language are views of a document; the version selects *which document* is being
edited, and every write endpoint loads the project fresh and resolves through
the stored `VersionID`. Left in memory, switching to 1.0 in the sidebar and
typing a headline wrote it into 1.1 — silently, with nothing on screen changing.

Documents written before versions existed keep their set under `screens` at the
project level. `Project.Migrate` folds that into version 1.0, and it runs on the
way out of the store *and* inside `Edit` — a patch reads the stored row
directly, so an unmigrated project edited without it would be written back with
its screens under the legacy key and an empty version.

The store listing is per `(version, language)`, the way App Store Connect's is —
promotional text, description, What's New, keywords, and the two URLs. Copyright
is per version and not per language, because Apple's is. A language with nothing
of its own falls back to the base one on the way *out*; never into the form,
which would save the base language's words into a language nobody has written.

## House style

Comments explain **why**, not what: the reason a line exists, the bug it
prevents, the measurement behind a number. Match the density of the file you are
editing. When a claim is measurable, measure it and put the number in the text.

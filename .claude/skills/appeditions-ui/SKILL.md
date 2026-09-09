---
name: appeditions-ui
description: Build or change AppEditions's interface — pages, layout, the step rail, shared components, icons, panels, the tune controls, Tailwind classes, shadcn-templ usage. Read this BEFORE editing anything under client/pages/ or client/ui/, adding a class to a .templ file, or building any overlay. Covers what belongs in the shell vs the layout vs a page (getting this wrong produces chrome that freezes on the first page loaded), how to build a panel with no JavaScript, why a CLOSED panel still costs render work unless you gate it, the icon registry, the aria-current rule, how a control reaches the server, and the two build steps that silently do nothing if skipped.
---

# AppEditions UI

Go + templ + Tailwind v4 + shadcn-templ v2. No React, no bundler, no client
framework. `CLAUDE.md` covers the product; this covers the interface.

## The shape of every interaction

There is no client-side model of a project. A control posts to an endpoint and
the page re-renders:

```
control --(fetch)--> endpoint --> store --> forge.js re-navigates --> server renders
```

`client/public/forge.js` binds by delegation on `document`, because a local
navigation replaces `#outlet` wholesale and a listener bound inside it dies on
the next render. The attributes it looks for:

| attribute | on | meaning |
| --- | --- | --- |
| `data-action="/api/…"` + `data-body='{…}'` | a button | post that body, then re-render |
| `data-form="/api/…"` | a form | post the form as JSON, then re-render |
| `data-form-multipart="/api/…"` | a form | post the form as `FormData` (it carries a file), then re-render |
| `data-patch` | any control | post `{key: name, value: value}` to the nearest `[data-patch-scope]` |
| `data-patch-scope="/api/…"` | a container | where its controls post |
| `data-patch-set` + `data-value` | a button | set one key to a fixed value |
| `data-copy` + `data-locale` + `data-field` | an input | save on blur |
| `data-upload` / `data-drop` | a file input / a zone | multipart upload |
| `data-panel-toggle="<id>"` | a button | open/close the panel whose checkbox has that id |
| `data-picks="<id>"` | a button | open the hidden file input with that id |
| `data-confirm="…"` | anything with `data-action` | confirm first |

**Never wrap a button in a `<label for="…">`.** A label does not forward
activation to an interactive element, so the click lands on the button and the
checkbox never flips — the panel silently does nothing and nothing says why.
Use `ui.PanelToggle(id)` and `ui.Picks(id)` on the button's `Attributes`.

Adding a control is adding markup. If you find yourself writing a new listener,
check whether one of these covers it first.

## Read first, in this order

1. `docs/shadcn-templ.md` — the offline digest of the **exact beta pinned here**.
   Do not guess React shadcn APIs and do not fetch the website. The Go API is
   different.
2. `howl_conventions` (MCP) section `"Routing conventions"` or
   `"Making a page interactive"` — the file-naming rules are not guessable,
   because Go rejects `_layout.templ` and `[id].templ`.

## Where a thing goes

This is the decision that goes wrong most, and it fails silently.

A client-side navigation swaps **`#outlet` and nothing else**. The document
shell is rendered once, by the server, on a cold load.

| put it in | when | file |
|---|---|---|
| **the page** | it is this route's content | `client/pages/<path>/index.templ` |
| **the layout** | it is chrome, and it depends on the route | `client/pages/layout.templ`, or `client/pages/projects/id.dyn/layout.templ` for the project workspace |
| **the shell** | it must outlive a navigation and does NOT depend on the route | `client/pages/app.templ` |
| **shared** | two pages render it | `client/ui/ui.templ` |

**Route-dependent markup in the shell freezes** on whichever page was loaded
first, and only a refresh corrects it. That is why the top navigation lives in
`layout.templ` and the step rail lives in the project layout. The shell keeps
only the document, the stylesheet and the scripts.

The exceptions that legitimately stay in the shell are maintained by the client
runtime: `aria-current` on every same-origin link is re-applied after each
navigation, and `document` fires `howl:navigate` for anything else.

## Modifiers

- `index.templ` — server-rendered. Every route here is one of these; there is no
  wasm build in this project.
- `*.raw.templ` — **its own document**: no shell, no layout. Not used yet.
- `*.bare.templ` — keeps the shell, drops the layout chain.
- `id.dyn/` — a `{id}` path parameter, as a *directory*, so the steps below it
  (`shots/`, `copy/`, `tune/`, `export/`) are ordinary nested routes. Read it
  with `router.Param(ctx, "id")`.

## Icons

One registry, not one templ per icon: `client/ui/icons.go`.

```templ
@ui.Icon("logs", "size-4")
```

Adding one is a row in `Icons`: the inner paths, plus only the attributes that
differ from the default (24×24, `stroke="currentColor"`, width 1.5, no fill). An
unknown name renders nothing — a missing icon should cost a missing icon.

Icons are `currentColor`, so they inherit the row's colour. Size them with the
class argument, never with a `width` attribute.

## The step rail

`ui.Steps` owns the order: Look, Screenshots, Copy, Fine-tune, Review & export.
The order is the order the decisions depend on — the look shapes the slots, the
slots take the screenshots, the copy sits on them — but **the rail is navigation
and never a gate**. Every step is one click away at any time, because someone
who wants to fix a headline before the last slot is filled is not doing it
wrong.

`ui.Rail` colours each row from `stepStatus`, which reads the same
`model.Readiness` the status line and the review page read. One reading, three
places: a second count would eventually disagree about whether the author may
export.

Colour is status and nothing else here — green is done, amber needs attention,
and the accent is reserved for the current step and the primary action. Do not
put the accent on ordinary chrome or none of that reads.

Links mark themselves with `aria-current` and style from it, never with a class
computed at render time.

## Panels and disclosures

AppEditions serves **no interactive shadcn bundle**, so dialog/popover/select are
not available. Every overlay here is native:

| want | use | example |
|---|---|---|
| a modal | `ui.Modal(id, title, hint)` + a button carrying `ui.PanelToggle(id)` | `newProjectModal` |
| a form that folds away in place | a checkbox carrying `data-panel`, a sibling `data-panel-body`, and a button carrying `ui.PanelToggle(id)` | `saveTemplatePanel`, `languagePanel` |
| a dropdown | a native `<select class="cn-native-select">` through `ui.NativeSelect` | every control in the tune panel |
| a swatch group | radios wrapped in labels, `peer-checked:` on the swatch | `ui.Swatch` |

No JavaScript, keyboard-operable, and nothing to re-hydrate after a navigation.
The open/closed rule lives unlayered in `app.css`, not as a utility class, so
nothing depends on how a `hidden` in `@layer utilities` cascades against it.

**The rule uses `+`, not `~`, and that is load-bearing.** The general sibling
combinator matches *every* following sibling, so on a page that renders one
modal per row — the projects page renders one per card — each unchecked
checkbox holds every modal after it shut, and only the first one in the
document can ever open. It looks like "some modals are broken". Tailwind's
`peer-checked:` has the same shape, so it has the same problem; keep the pair
adjacent and the combinator adjacent.

`ui.Modal` is the same mechanism with a fixed overlay: the backdrop is a
`<label>` for the same checkbox, `body:has([data-modal]:checked)` locks the page
behind it, and `forge.js` adds Escape — the one part a label cannot express.

It is a flex column capped at `max-h-full`, so a form taller than the window
scrolls inside it with the title and the actions staying put. `min-h-0` on the
scrolling middle is load-bearing: without it a flex child refuses to shrink
below its content and the cap does nothing, which is how a modal ends up
running off the bottom of the screen with its own submit button. The footer is
a separate slot, so its buttons sit outside the form and carry
`form="<the form's id>"` — which is also why `forge.js` reads
`event.submitter` rather than searching the form's subtree for the button.
Use it for a form with consequences (the new-project form sets the store size
and the base language, both painful to change later); use a panel for something
edited in place.

**A modal cannot live inside a transformed ancestor.** `position: fixed`
resolves against the nearest ancestor with a transform, not the viewport — so a
modal rendered inside a card that carries `hover:-translate-y-0.5` is trapped
inside that card. Render per-row modals after the list, not in the row.

**Every action inside a modal closes it**, because `post()` re-renders `#outlet`
and the modal's checkbox comes back unchecked. That is right for Save and
Delete, and it is why the edit form carries its file *and* its text in one
multipart request: uploading the icon separately would re-render between the two
and discard whatever had been typed.

**Anything inside a card needs `card.Content`.** `cn-card` carries
`padding-block` and no horizontal padding at all, so a form as a direct child of
`card.Card` sits flush against the left and right borders.

### Gate whatever fills a closed panel

A closed panel is `display:none`, and **`display:none` markup is still present**
to every `querySelector` and every `<img>` the browser decides to fetch.

Here that matters most for preview tiles: every one of them is a request that
renders a PNG on the server. A strip of tiles inside a shut panel would render
the whole set for nobody. `loading="lazy"` on `ui.Tile` covers the common case;
anything expensive that is not an image needs an explicit check on whether the
panel's checkbox is checked.

Applies to anything conditionally hidden, not just panels: a tab that is not the
current tab, a shut `<details>`, an off-canvas drawer.

### Forms

A `required` field inside a hidden panel is one the browser refuses to focus and
refuses to explain — the form silently will not submit and nothing says why. If
a panel can be shut, either keep its required fields out of it or drop the
`required`.

## The window is not a browser

Two things behave differently in the WKWebView, and both fail silently:

- **`file.type` is often empty.** The window builds its `File` objects from the
  URLs the open panel returned and does not always resolve a MIME type, so a
  filter written as `file.type.startsWith("image/")` throws away every file the
  author picked. `looksLikeImage` in `forge.js` falls back to the extension and
  lets the server — which decodes every upload anyway — be the authority.
- **Drag-and-drop is preventDefault-ed at the window.** howl-go injects a guard
  so a dropped file cannot navigate the webview to `file://`. Handlers on
  `document` still see the event first, which is why `data-drop` works; one
  bound on `window` would not.

## Tailwind: the two things that silently do nothing

1. **`client/public/app.css` is a build artifact and is committed.** Tailwind
   only emits classes it can *find*. Use a class nothing used before and you
   must run `make css` — the one target that needs Node. Verify it landed:
   `grep -o '\.your-class' client/public/app.css`. Run it **before** `make`: the
   binary embeds `client/public`, so a stylesheet rebuilt afterwards is one the
   server does not serve.
2. **A class assembled from a variable is never emitted at all.**
   `` `col-span-${n}` `` produces nothing. Use an inline style, or enumerate the
   variants literally.

A new file under `client/public/*.js` needs its own `@source` line in the `css`
target or nothing in it is scanned.

Load-bearing: `<html class="dark style-nova">`. `style-nova.css` nests every
`cn-*` rule under `.style-nova`, so dropping it unstyles every shadcn component
at once.

## JavaScript

`forge.js` binds by **delegation on `document`** (`event.target.closest("[data-…]")`),
which is why its controls keep working after every re-render. Keep it that way:
a listener bound to a specific element in the outlet dies on the next
navigation.

The one place the client draws before the server has answered is the number
beside a slider (`data-live`). Everything visual comes from the renderer; if you
find yourself about to compute a colour or a layout in JavaScript, it belongs in
`internal/render` instead.

For state that must react in the shell, listen for `howl:navigate` on
`document` — nothing outside `#outlet` is re-rendered.

## Before you finish

```bash
make css        # FIRST, and only if you used a class that did not exist before
make            # fsapis -> fsroutes -> templ generate -> build (in that order, always)
make check      # howl check
```

`howl check` reports the browser-side mistakes that fail silently: a `Mount`
that subscribes with no `Unmount`, a discarded `dom.On` release func, a
`@pkg.Component()` the package does not declare.

## House style

Comments explain **why**, not what: the reason a line exists, the bug it
prevents, the measurement behind a number. Match the density of the file you are
editing. When a claim is measurable, measure it and put the number in the text.

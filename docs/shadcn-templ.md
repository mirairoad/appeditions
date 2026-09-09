# shadcn-templ offline agent guide

AppEditions pins `github.com/axadrn/shadcn-templ/v2@v2.0.0-beta.3` and its whole interface is built from it. Read this before changing the UI. It summarizes the upstream documentation and this project's integration so routine work does not require fetching templui.io or shadcn-templ.com. A verbatim snapshot of the pinned core guides and all 52 component pages lives in `docs/shadcn-templ-upstream/`.

This file was adapted from the same guide in [guard](https://github.com/hushkey-app/guard), which pins the same version.

## What it is

shadcn-templ is the Go/templ pendant of shadcn/ui: accessible templ components, Tailwind CSS styling, and vanilla JavaScript for interactive behavior. It is MIT licensed. Version 2 is beta; pin exact versions and expect API changes.

The supported workflow copies component source into the application with the CLI. The direct Go import workflow this project uses is explicitly experimental upstream. It works for static components; see "Interactive components" for the part that does not.

## AppEditions integration

- Every page imports shadcn-templ packages directly.
- `client/styles/app.css` is the stylesheet source: Tailwind, the three upstream imports, AppEditions's theme tokens on `:root`, and a short unlayered block for the preview tiles, the drop zone and the disclosure panels.
- `make css` compiles it to `client/public/app.css`, which is committed, so `make`, `make dev` and `go test` need no Node/npm. It also writes the gitignored `client/styles/app.sources.css`, which carries this machine's module-cache path.
- **Run `make css` before `make`, never after.** The binary embeds `client/public`, so a stylesheet rebuilt after the go build is one the server does not serve.
- Tailwind only emits classes it finds in the `@source` globs — `client/pages/**/*.templ`, `client/ui/**/*.templ`, and `client/public/forge.js`. **A class used only in a dynamically built string in forge.js must be written out in full**, or it will not exist in the bundle. `forge.js` builds the toast's classes that way; they are spelled out for this reason.
- `<html class="dark style-nova">`. `style-nova.css` nests every `cn-*` rule under `.style-nova`; without that class every component renders unstyled but structurally correct, which is a confusing failure. `dark` is what the `dark:` variants key off.
- The chrome is dark deliberately: the work on screen is a set of bright store tiles, and a dark frame is the only way the tile's own background reads as the artwork rather than as more chrome.
- `style-nova` selects the component style. Other upstream styles are Vega, Maia, Lyra, Mira, Luma, Sera, and Rhea. Changing style means changing both the imported `style-*.css` in the Make target and the class on `<html>`.
- `.cn-native-select` exists in the stylesheet but has no Go component. `ui.NativeSelect` uses it for every dropdown in the tune panel.
- shadcn's `Input` generates a random ID when none is given, which would differ between two renders of the same page. Always pass `ID`.

## Rendering compatibility

Static components are ordinary `templ.Component`s. They work with howl-go cold SSR, partial navigation and static export. AppEditions renders every page on the server; there is no wasm build here.

Interactive components use a shared vanilla-JavaScript bundle and DOM data attributes. AppEditions does not enable it: the v2 direct-import workflow is experimental, and its development handler looks for a locally copied `components/` directory. Dialog, select, popover, tooltip and similar controls are therefore off the table until a component is CLI-copied into the repo.

Everything that has to be interactive here is either a native element driven by `forge.js` or a checkbox/radio with a sibling selector. That is not only a workaround: an overlay built from a checkbox has nothing to re-hydrate after a navigation, and a native `<select>` is keyboard- and screen-reader-correct for free.

## Universal component API

Most components accept:

| Prop | Type | Meaning |
| --- | --- | --- |
| `ID` | `string` | Rendered element ID |
| `Class` | `string` | Additional classes merged with defaults |
| `Attributes` | `templ.Attributes` | Arbitrary HTML, data and ARIA attributes |

Components compose through templ children:

```templ
@button.Button(button.Props{
    Variant: button.VariantOutline,
    Attributes: templ.Attributes{"aria-label": "Refresh"},
}) {
    Refresh
}
```

## Components exercised here

### Button

Import `github.com/axadrn/shadcn-templ/v2/components/button`.

Variants: `VariantDefault`, `VariantSecondary`, `VariantOutline`, `VariantGhost`, `VariantDestructive`, `VariantLink`.

Sizes: `SizeDefault`, `SizeXs`, `SizeSm`, `SizeLg`, `SizeIcon`, `SizeIconXs`, `SizeIconSm`, `SizeIconLg`.

Set `Href` to render an anchor. Types are `TypeButton`, `TypeSubmit`, and `TypeReset`; the default is `TypeButton`.

### Input and Textarea

Import `github.com/axadrn/shadcn-templ/v2/components/input` and `.../textarea`.

`input.Props`: `Name`, `Type`, `Value`, `Form`, `Placeholder`, `Disabled`, `ReadOnly`, `Required`, `Accept`. Textarea adds `Rows`. `min`/`max`/`autocomplete` and other native attributes go through `Attributes`. Always set `ID` (see the integration notes above).

### Card

Import `github.com/axadrn/shadcn-templ/v2/components/card`.

`card.Card` contains `card.Header`, `card.Title`, `card.Description`, optional `card.Action`, `card.Content`, and `card.Footer`. AppEditions's project and template cards pass `Class: "gap-0 overflow-hidden py-0"` to cancel the card's own padding, because the thumbnail bleeds to the edge.

### Badge

Badge variants mirror the common shadcn variants. style-nova has no warning or success badge, and AppEditions needs both for readiness state; `ui.Count` builds them from theme tokens (`bg-warning/15 text-warning`, `bg-success/15 text-success`) on top of the plain `cn-badge` base rather than forking the component.

### Textarea

Import `github.com/axadrn/shadcn-templ/v2/components/textarea`. Same props as Input plus `Rows`.

## Interactive components

The copied-source workflow renders `@components.Scripts()` once in the application shell and mounts `components.ScriptsHandler()` at `GET /components/{bundle}`. Do not add per-page script tags. The bundle contains each installed component's IIFE and is intended to handle dynamic DOM swaps.

AppEditions does not run this bundle. Before adopting interactive controls, verify:

1. Cold-load initialization.
2. Navigation into the route through Howl partial navigation.
3. Navigation away and back without duplicate listeners or stale portals.
4. Escape, focus trap, focus return, outside click, and screen-reader labels.
5. CSP nonce behavior.
6. Production and development bundle serving.

## Customization

There are three levels:

1. Theme tokens in `client/styles/app.css`.
2. `Class` and `Attributes` at each use site. `Class` is merged with tailwind-merge, so `card.Props{Class: "py-0 gap-0"}` really does cancel the card's own padding.
3. CLI-copy a component into the repo when its structure or behavior must change. This is the supported shadcn ownership model.

Do not edit the Go module cache. Copy selected components into a leaf package such as `client/components` and commit them.

## Commands

```bash
make css   # rebuild client/public/app.css — the only target that needs Node
make       # endpoints + routes + templ generation + the binary
make test
```

`make css` writes the gitignored `client/styles/app.sources.css` with the resolved module-cache path, then compiles `client/styles/app.css`. The output is committed so normal builds and CI remain offline.

## Upstream provenance

This guide was derived from documentation and source in the pinned v2.0.0-beta.3 module:

- `internal/service/content/docs/introduction.md`
- `internal/service/content/docs/installation.md`
- `internal/service/content/docs/import-workflow.md`
- `internal/service/content/docs/components/*.md`
- `components/*/*.templ` and `components/*/*.js`

When upgrading, review those local module files and update this guide with the code.

.PHONY: all generate apis run run-debug serve dev dev-web desktop run-desktop package css test check clean

MODULE := github.com/mirairoad/appeditions
BIN    := appeditions
DESKTOP := appeditions-desktop

# Packaging. VERSION is what Get Info shows; APP_ID is what Launch Services
# keys its icon and permission caches on, so it must not change once a build
# has been installed anywhere — a rebuilt app under a new identifier leaves a
# ghost behind that still claims the old icon.
VERSION ?= 0.1.0
APP_NAME := AppEditions
APP_ID   := com.mirairoad.appeditions

# The commit this binary was built from, stamped into it so the app can tell
# whether it is behind the repository. That is the version that matters for
# updates: there are no releases and no tags, install.sh builds whatever main
# points at, and running it again is how you update. A tree with no git — a
# source tarball — leaves it "dev" and the check does not run at all.
COMMIT  := $(shell git rev-parse --short=7 HEAD 2>/dev/null || echo dev)
LDFLAGS := -X $(MODULE)/boot.Version=$(VERSION) -X $(MODULE)/boot.Commit=$(COMMIT)

# fsroutes -> fsapis -> templ -> build, always in that order. The route table
# and the API table are generated into the tree that templ then compiles, so a
# generate step out of order builds the previous revision's routes.
all: generate
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

generate: apis
	go run github.com/mirairoad/howl-go/core/cmd/fsroutes -module $(MODULE)/client/pages
	go tool templ generate

apis:
	go run github.com/mirairoad/howl-go/core/cmd/fsapis -dir server/apis \
		-module $(MODULE)/server/apis -client client/api/api_gen.go -client-pkg apiclient

# The app in a window. This is the one to run.
run: desktop
	./$(DESKTOP)

run-desktop: run

# The window with its devtools, and its server on a port a browser can open.
#
# -debug turns the inspector on; since macOS 13.3 that also needs
# WKWebView.inspectable, which desktop/filepanel_darwin.m sets. Attach from
# Safari ▸ Develop ▸ this Mac ▸ AppEditions, or right-click ▸ Inspect Element.
#
# -addr puts the *same session* on :9010 as well, so the window and a browser
# can be pointed at one server and compared. That is how the preview-tile
# caching difference between WebKit and Chromium was found.
run-debug: desktop
	./$(DESKTOP) -debug -addr :9010

# Just the server, for looking at it in a browser instead.
serve: all
	./$(BIN)

# The development loop: watch, rebuild, restart, and reload the window in
# place. The window carries -debug, so the inspector is there when a change
# does not look the way it should; `howl dev` is already serving :9010, so the
# same session is open in a browser at the same time.
#
# `howl dev` restarts the server binary on every save but keeps its proxy port
# up, so the window connects once and reloads its content rather than being
# killed and respawned. The window must NOT be howl dev's child for that
# reason. howl itself is built rather than `go run`, because the trap has to
# kill the dev server and `go run` would leave it orphaned behind its wrapper.
#
# fsapis runs in the -pre step because it writes into the tree the build then
# compiles.
dev: desktop
	@go build -o /tmp/appeditions-howl github.com/mirairoad/howl-go/core/cmd/howl
	@/tmp/appeditions-howl dev -dir . -addr :9010 \
		-pre "go run github.com/mirairoad/howl-go/core/cmd/fsapis -dir server/apis -module $(MODULE)/server/apis -client client/api/api_gen.go -client-pkg apiclient" & \
		dev=$$!; trap "kill $$dev 2>/dev/null" EXIT INT TERM; \
		./$(DESKTOP) -debug -attach :9010

# The same loop without the window, for working on the markup in a browser.
dev-web:
	go run github.com/mirairoad/howl-go/core/cmd/howl dev -addr :9010 \
		-pre "go run github.com/mirairoad/howl-go/core/cmd/fsapis -dir server/apis -module $(MODULE)/server/apis -client client/api/api_gen.go -client-pkg apiclient"

# The window is a nested module: it needs cgo and a webview binding, so a
# machine without WebKitGTK still builds everything else.
#
# The nested module links this one's boot package, so the same -X paths reach
# it: the flags name a package, not a module.
desktop: all
	cd desktop && go build -ldflags "$(LDFLAGS)" -o ../$(DESKTOP) .

# The desktop binary as something the OS shows an icon for: a .app on macOS, a
# .desktop entry plus hicolor PNGs and an install.sh on Linux. Both are written
# to dist/ from desktop/packaging/icon.png, in pure Go, so there is no
# iconutil/sips step.
#
# Select the target with `make package TARGET=linux`, never GOOS=linux: make
# exports a command-line variable into the environment of every recipe, so GOOS
# cross-compiles fsapis and the build dies trying to run it. The Linux tree has
# to be packaged on Linux in any case — the desktop binary needs cgo and the
# platform's webview, and cannot be cross-compiled at all. `howl package`
# refuses a mismatch rather than writing a tree that installs, appears in the
# launcher and does nothing when clicked.
package: desktop
	@go run github.com/mirairoad/howl-go/core/cmd/howl package \
		-bin ./$(DESKTOP) -name "$(APP_NAME)" -id $(APP_ID) -version $(VERSION) \
		$(if $(TARGET),-os $(TARGET))

# Tailwind, as the standalone binary: one file, no Node, no npm. This is what
# shadcn-templ's own installation notes call for, and it is pinned because v2 is
# beta and the upstream advice is to pin exact versions.
#
# Cached under .howl/, which is gitignored, and named for the version so a bump
# fetches rather than silently reusing the old compiler. Point TAILWIND at a
# binary you already have to skip the download entirely.
TAILWIND_VERSION := 4.1.18
TAILWIND ?= .howl/tailwind/tailwindcss-$(TAILWIND_VERSION)

$(TAILWIND):
	@mkdir -p $(dir $@)
	@set -e; \
	case "$$(uname -s)" in \
		Darwin) os=macos ;; \
		Linux)  os=linux ;; \
		*) echo "no Tailwind standalone build for $$(uname -s)" >&2; exit 1 ;; \
	esac; \
	case "$$(uname -m)" in \
		arm64|aarch64) arch=arm64 ;; \
		x86_64|amd64)  arch=x64 ;; \
		*) echo "no Tailwind standalone build for $$(uname -m)" >&2; exit 1 ;; \
	esac; \
	libc=; \
	if [ "$$os" = linux ] && ldd --version 2>&1 | grep -qi musl; then libc=-musl; fi; \
	url="https://github.com/tailwindlabs/tailwindcss/releases/download/v$(TAILWIND_VERSION)/tailwindcss-$$os-$$arch$$libc"; \
	echo "fetching $$url"; \
	curl -fsSL --retry 3 -o $@.part "$$url"; \
	chmod +x $@.part; \
	mv $@.part $@

# The only target that writes client/public/app.css — which is committed, so
# every other target and CI stay offline. Run it BEFORE `make`, not after: the
# binary embeds client/public, so a stylesheet rebuilt after the go build is one
# the server does not serve.
#
# Tailwind emits only the classes it can find. A class used for the first time,
# or a new .js file under client/public, needs this target re-run; a class
# assembled from a variable is never emitted at all.
css: $(TAILWIND)
	@SHADCN="$$(go list -mod=mod -m -f '{{.Dir}}' github.com/axadrn/shadcn-templ/v2)"; \
	printf '%s\n' \
		"@import \"$$SHADCN/assets/css/tw-animate.css\";" \
		"@import \"$$SHADCN/assets/css/shadcn-tailwind.css\";" \
		"@import \"$$SHADCN/assets/css/styles/style-nova.css\" layer(base);" \
		'@source "../pages/**/*.templ";' \
		'@source "../ui/**/*.templ";' \
		'@source "../public/forge.js";' \
		"@source \"$$SHADCN/components/**/*.templ\";" \
		> client/styles/app.sources.css
	@$(TAILWIND) -i client/styles/app.css -o client/public/app.css --minify

test:
	go test ./...

# The conventions, enforced: a page importing core/app or db, a Mount with no
# Unmount, a component call the package does not declare.
check:
	go run github.com/mirairoad/howl-go/core/cmd/howl check

clean:
	rm -f $(BIN) $(DESKTOP)

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

# fsroutes -> fsapis -> templ -> build, always in that order. The route table
# and the API table are generated into the tree that templ then compiles, so a
# generate step out of order builds the previous revision's routes.
all: generate
	go build -o $(BIN) .

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
desktop: all
	cd desktop && go build -o ../$(DESKTOP) .

# The desktop binary as something the OS shows an icon for: a .app on macOS, a
# .desktop entry plus hicolor PNGs and an install.sh on Linux. Both are written
# to dist/ from desktop/packaging/icon.png. Cross-package with `make package
# GOOS=linux`, which works from either platform because nothing here shells out
# to iconutil or sips.
package: desktop
	@go run github.com/mirairoad/howl-go/core/cmd/howl package \
		-bin ./$(DESKTOP) -name "$(APP_NAME)" -id $(APP_ID) -version $(VERSION) \
		$(if $(GOOS),-os $(GOOS))

# The only target that needs Node, and the only one that writes
# client/public/app.css — which is committed, so every other target and CI stay
# offline. Run it BEFORE `make`, not after: the binary embeds client/public, so
# a stylesheet rebuilt after the go build is one the server does not serve.
#
# Tailwind emits only the classes it can find. A class used for the first time,
# or a new .js file under client/public, needs this target re-run; a class
# assembled from a variable is never emitted at all.
css:
	@npm install --no-save --no-package-lock --prefix .howl/tailwind \
		tailwindcss@4.1.18 @tailwindcss/cli@4.1.18 >/dev/null
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
	.howl/tailwind/node_modules/.bin/tailwindcss -i client/styles/app.css -o client/public/app.css --minify

test:
	go test ./...

# The conventions, enforced: a page importing core/app or db, a Mount with no
# Unmount, a component call the package does not declare.
check:
	go run github.com/mirairoad/howl-go/core/cmd/howl check

clean:
	rm -f $(BIN) $(DESKTOP)

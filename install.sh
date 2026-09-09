#!/bin/sh
# Build AppEditions from source, install it, and delete the source.
#
# The build needs Go and a C toolchain. The installed app needs neither — it is
# one static-except-for-the-webview binary with the fonts and the stylesheet
# embedded — so there is nothing worth keeping afterwards, and this clones into
# a temporary directory that is removed on the way out however the script ends.
#
# Running it again is how you update: it always builds whatever REF points at.
#
#   curl -fsSL https://raw.githubusercontent.com/mirairoad/appeditions/main/install.sh | sh
#
# Environment:
#   REPO      clone from somewhere else (a fork, or a local path)
#   REF       branch, tag or commit to build            (default main)
#   PREFIX    Linux install root                        (default $HOME/.local)
#   APPDIR    macOS install directory                   (default /Applications)
#   KEEP_SRC  1 to leave the build tree behind and print where it is
set -eu

REPO="${REPO:-https://github.com/mirairoad/appeditions.git}"
REF="${REF:-main}"
KEEP_SRC="${KEEP_SRC:-0}"

say() { printf '==> %s\n' "$*"; }
die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

# Go 1.25 is the module's own minimum: an older toolchain refuses the build
# rather than producing something subtly wrong, so check it here where the
# message can say what to do about it.
check_go() {
	command -v go >/dev/null 2>&1 || die "Go 1.25 or newer is required: https://go.dev/dl/"
	v=$(go env GOVERSION 2>/dev/null || echo "")
	v=${v#go}
	major=${v%%.*}
	rest=${v#*.}
	minor=$(printf '%s' "${rest%%.*}" | tr -cd '0-9')
	[ -n "$major" ] && [ -n "$minor" ] || return 0 # unparseable: let the build decide
	if [ "$major" -lt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -lt 25 ]; }; then
		die "Go 1.25 or newer is required, found $(go env GOVERSION)"
	fi
}

# The window is cgo over the platform's webview, so the Linux build needs the
# GTK development headers. Without them cgo fails inside the vendored binding
# with an error that names a header nobody has heard of, which is a bad first
# impression of a build that is otherwise one command.
check_linux_deps() {
	command -v cc >/dev/null 2>&1 || command -v gcc >/dev/null 2>&1 ||
		die "a C compiler is required (apt install build-essential, dnf install gcc)"
	command -v pkg-config >/dev/null 2>&1 ||
		die "pkg-config is required (apt install pkg-config, dnf install pkgconf-pkg-config)"
	if ! pkg-config --exists webkit2gtk-4.1 && ! pkg-config --exists webkit2gtk-4.0; then
		die "WebKitGTK development files are required:
  Debian/Ubuntu  sudo apt install libwebkit2gtk-4.1-dev libgtk-3-dev
  Fedora         sudo dnf install webkit2gtk4.1-devel gtk3-devel
  Arch           sudo pacman -S webkit2gtk-4.1 gtk3"
	fi
}

command -v git >/dev/null 2>&1 || die "git is required"
command -v make >/dev/null 2>&1 || die "make is required"
check_go

os=$(uname -s)
case "$os" in
Linux) check_linux_deps ;;
Darwin) ;;
*) die "unsupported platform: $os (macOS and Linux only)" ;;
esac

# Everything below happens inside this directory and nowhere else, so running
# the script from within a working copy cannot delete that working copy.
tmp=$(mktemp -d 2>/dev/null || mktemp -d -t appeditions)
cleanup() {
	if [ "$KEEP_SRC" = 1 ]; then
		say "source kept at $tmp"
	else
		rm -rf "$tmp"
	fi
}
trap cleanup EXIT INT TERM

src="$tmp/src"
say "fetching $REF from $REPO"
# --depth 1 for the usual case; a raw commit cannot be cloned by name, so fall
# back to a full clone and check it out.
if ! git clone --quiet --depth 1 --branch "$REF" "$REPO" "$src" 2>/dev/null; then
	rm -rf "$src" # a refused --branch can still leave the directory behind
	git clone --quiet "$REPO" "$src" 2>/dev/null || die "could not clone $REPO"
	git -C "$src" checkout --quiet "$REF" 2>/dev/null ||
		die "no such branch, tag or commit in $REPO: $REF"
fi

rev=$(git -C "$src" rev-parse HEAD)
described=$(git -C "$src" describe --tags --always 2>/dev/null || echo "")

# VERSION is what Get Info shows, so pass a real version when the checkout has
# one and leave the Makefile's default alone when `describe` only has a commit
# to offer.
set -- package
case "$described" in
v[0-9]*) set -- "$@" "VERSION=${described#v}" ;;
[0-9]*) set -- "$@" "VERSION=$described" ;;
esac
# TARGET, never GOOS: make exports a command-line variable into the environment
# of every recipe, and GOOS there cross-compiles the code generators, which the
# build then cannot run.
[ "$os" = Linux ] && set -- "$@" TARGET=linux

say "building (this takes a minute on a cold module cache)"
# Quiet unless it fails: cgo warns about the vendored webview header on every
# build, and a wall of warnings from a successful build reads like a problem.
# The log lives in $tmp, so print it before the trap takes it away.
log="$tmp/build.log"
if ! (cd "$src" && make "$@") >"$log" 2>&1; then
	cat "$log" >&2
	die "build failed"
fi

case "$os" in
Darwin)
	APPDIR="${APPDIR:-/Applications}"
	if ! [ -w "$APPDIR" ]; then
		APPDIR="$HOME/Applications"
		say "/Applications is not writable, installing to $APPDIR"
		mkdir -p "$APPDIR"
	fi
	# Replacing rather than merging: a stale file left inside an old bundle is
	# how you get a version that launches and behaves like neither build.
	[ -d "$APPDIR/AppEditions.app" ] && rm -rf "$APPDIR/AppEditions.app"
	cp -R "$src/dist/AppEditions.app" "$APPDIR/"
	installed="$APPDIR/AppEditions.app"
	;;
Linux)
	PREFIX="${PREFIX:-$HOME/.local}"
	export PREFIX
	"$src/dist/appeditions/install.sh"
	installed="$PREFIX/bin/appeditions"
	# Keep the generated script: its --uninstall branch is the only thing that
	# knows every path it wrote, and the tree it came in is about to be deleted.
	# That branch reads PREFIX and the names, never its own directory, so a copy
	# on its own still works.
	mkdir -p "${XDG_STATE_HOME:-$HOME/.local/state}/appeditions"
	cp "$src/dist/appeditions/install.sh" \
		"${XDG_STATE_HOME:-$HOME/.local/state}/appeditions/uninstall.sh"
	;;
esac

# What was built, so update.sh can tell whether there is anything to do without
# cloning twelve megabytes of fonts to find out.
state="${XDG_STATE_HOME:-$HOME/.local/state}/appeditions"
mkdir -p "$state"
printf '%s\n' "$rev" >"$state/source-revision"

say "installed $installed"
[ -n "$described" ] && say "from $described ($(printf '%.7s' "$rev"))"
case "$os" in
Linux) say "if it is not in your launcher yet, log out and back in, or run $PREFIX/bin/appeditions" ;;
Darwin) say "open it from $APPDIR, or run: open -a AppEditions" ;;
esac
say "your projects live in ~/.appeditions and were not touched"

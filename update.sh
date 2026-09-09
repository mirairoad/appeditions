#!/bin/sh
# Rebuild AppEditions from the current tip and reinstall it.
#
# install.sh is the whole implementation. Because the source is deleted after a
# build there is nothing here to `git pull` — an update is the same operation as
# an install, and all this adds is the check that skips it when the tip is
# already what you are running.
#
#   curl -fsSL https://raw.githubusercontent.com/mirairoad/appeditions/main/update.sh | sh
#
# Environment: as install.sh, plus
#   FORCE  1 to rebuild even when the installed revision is already current
set -eu

REPO="${REPO:-https://github.com/mirairoad/appeditions.git}"
REF="${REF:-main}"
RAW="${RAW:-https://raw.githubusercontent.com/mirairoad/appeditions/main/install.sh}"
FORCE="${FORCE:-0}"

say() { printf '==> %s\n' "$*"; }
die() { printf 'update.sh: %s\n' "$*" >&2; exit 1; }

command -v git >/dev/null 2>&1 || die "git is required"

state="${XDG_STATE_HOME:-$HOME/.local/state}/appeditions/source-revision"
have=""
[ -f "$state" ] && have=$(cat "$state")

# One network round trip, no clone: ls-remote answers with the tip's sha.
#
# First line only. A name like "main" matches every ref that ends in it —
# refs/heads/main and refs/remotes/origin/main both come back from a repository
# that has a remote — and two lines of sha never compare equal to the one line
# in the receipt, so the up-to-date check could never fire. ls-remote sorts by
# refname, which puts refs/heads first.
want=$(git ls-remote "$REPO" "$REF" 2>/dev/null | awk 'NR == 1 { print $1 }')
# A raw commit is not a ref and ls-remote says nothing about it; take REF at
# its word and let the clone fail later if it does not exist.
if [ -z "$want" ]; then
	case "$REF" in
	*[!0-9a-f]* | "") die "could not reach $REPO" ;;
	*) want="$REF" ;;
	esac
fi

if [ "$FORCE" != 1 ] && [ -n "$have" ] && [ "$have" = "$want" ]; then
	say "already on $(printf '%.7s' "$have") — nothing to do (FORCE=1 to rebuild anyway)"
	exit 0
fi

if [ -n "$have" ]; then
	say "$(printf '%.7s' "$have") -> $(printf '%.7s' "$want")"
else
	say "no installed revision recorded, building $(printf '%.7s' "$want")"
fi

# Beside this script when run from a clone, fetched when run through a pipe.
#
# The test is that $0 is itself a file. Piped, $0 is the shell's own name, and
# dirname of that is "." — so a bare directory test would run whatever
# install.sh happened to be in the caller's working directory, which is not
# this project's and has not been vouched for by anything.
here=""
if [ -f "$0" ]; then
	here=$(CDPATH= cd -- "$(dirname -- "$0")" 2>/dev/null && pwd) || here=""
fi
if [ -n "$here" ] && [ -x "$here/install.sh" ]; then
	REF="$REF" REPO="$REPO" exec "$here/install.sh"
fi

command -v curl >/dev/null 2>&1 || die "curl is required to fetch install.sh"
curl -fsSL "$RAW" | REF="$REF" REPO="$REPO" sh

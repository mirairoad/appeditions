// Package update answers one question: is this build behind the repository it
// came from?
//
// There are no releases and no tags — install.sh builds whatever `main` points
// at, and running it again is how you update. So "the version" of a build is
// the commit it was built from, and being out of date is that commit not being
// the head of main any more. Anything tag-shaped would be inventing a release
// process the project does not have.
package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Repo is the one thing this package will ever ask about, spelled out here
// rather than taken from a request: the endpoint that opens it hands a URL to
// the operating system, and a constant cannot be pointed somewhere else.
const (
	Repo = "mirairoad/appeditions"
	URL  = "https://github.com/" + Repo

	// The sha endpoint answers with the bare commit hash instead of a JSON
	// document, which is the whole response this needs.
	api    = "https://api.github.com/repos/" + Repo + "/commits/main"
	accept = "application/vnd.github.sha"

	// Script is the project's own updater. It compares the revision install.sh
	// recorded against the tip and re-runs install.sh when they differ, which
	// is the entire update story here — there are no release binaries, and the
	// source is deleted after a build, so there is nothing to `git pull`.
	//
	// Fetched rather than run from disk: an install keeps only uninstall.sh,
	// so update.sh is not there afterwards. This is the line the project
	// documents, executed instead of printed.
	Script = "https://raw.githubusercontent.com/" + Repo + "/main/update.sh"
)

// State is what the interface draws from. Behind is false whenever the answer
// is not known — no network, a rate limit, a build with no commit stamped in
// it — because "there might be an update" is not worth a button.
type State struct {
	Commit string
	Latest string
	Behind bool
}

// A Checker holds the last answer. It is read on every page render and written
// by one goroutine every few hours, which is what the mutex is for.
type Checker struct {
	mu    sync.RWMutex
	state State
}

func New(commit string) *Checker {
	return &Checker{state: State{Commit: commit}}
}

func (c *Checker) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Run checks now and then every six hours until the context ends.
//
// A goroutine rather than something the page loader does, because a render
// must not wait on api.github.com: the answer is a button that can appear one
// navigation later, and a request that hangs would hang the editor instead.
//
// Six hours because this is a desktop tool somebody leaves open for days, and
// the thing being watched changes at the pace a person pushes commits.
func (c *Checker) Run(ctx context.Context) {
	// A build with nothing stamped in it is a `go run` or a `make` from a tree
	// with no git: there is no commit to be behind.
	if c.State().Commit == "" || c.State().Commit == "dev" {
		return
	}

	tick := time.NewTicker(6 * time.Hour)
	defer tick.Stop()
	for {
		c.check(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// check asks once. Every failure is silent: this is a convenience, and an
// error banner about GitHub on top of somebody's screenshots would be noise
// about a thing they did not ask for.
func (c *Checker) check(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", accept)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return
	}
	// A sha is 40 characters; the cap is there because this reads a response
	// from the network into memory and the size of it is not this program's
	// decision.
	body, err := io.ReadAll(io.LimitReader(res.Body, 64))
	if err != nil {
		return
	}
	c.set(strings.TrimSpace(string(body)))
}

// running guards the one thing here that must not happen twice at once.
// Rebuilding is a minute of somebody's machine and it ends by replacing the
// binary this process was launched from; two of them racing to write it is not
// a state worth reasoning about.
var running atomic.Bool

// Apply rebuilds and reinstalls, and blocks until it has.
//
// It blocks on purpose. The alternative is a job with a status somewhere and a
// page that polls it, and this application has neither — what it has is a
// button that shows a busy state until its request answers, the same as the
// export does. The cost is a request that takes about a minute on a cold
// module cache, which nothing here times out.
//
// The new build is on disk when this returns, but this process is still the
// old one: a running program is not replaced by overwriting the file it was
// loaded from. Relaunching is the last step and only the person at the window
// can take it, so the message says so rather than pretending otherwise.
func Apply(ctx context.Context) (string, error) {
	switch runtime.GOOS {
	case "darwin", "linux":
	default:
		return "", fmt.Errorf("no updater for %s", runtime.GOOS)
	}
	if !running.CompareAndSwap(false, true) {
		return "", errors.New("an update is already running")
	}
	defer running.Store(false)

	// Generous, because the first build on a machine downloads the module
	// cache. A limit at all, because a wedged network should not leave a
	// goroutine and a shell behind for the life of the process.
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sh", "-c", "curl -fsSL "+Script+" | sh").CombinedOutput()
	if err != nil {
		// The script's own last words: it says which dependency is missing or
		// which step failed, and that is the only useful thing to show.
		return "", fmt.Errorf("%s", lastLine(out))
	}
	return lastLine(out), nil
}

// lastLine is the final thing the script said — "installed …", or the reason
// it stopped. Scripts here report with `==> ` lines, so the tail is the
// outcome and everything before it is progress.
func lastLine(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(strings.TrimPrefix(lines[i], "==>")); s != "" {
			return s
		}
	}
	return "the updater said nothing"
}

// set records an answer. The comparison is a prefix because the build stamps
// the short sha and the API returns the full one; comparing the other way
// round would report every build as out of date.
func (c *Checker) set(latest string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.Latest = latest
	c.state.Behind = latest != "" && !strings.HasPrefix(latest, c.state.Commit)
}

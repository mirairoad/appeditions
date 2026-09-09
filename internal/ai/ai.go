// Package ai writes and translates screenshot copy by shelling out to a
// coding agent the author already has installed.
//
// There is no API key here on purpose. Both supported providers are CLIs that
// carry their own authentication — `claude` and `codex` — so the app inherits
// whatever subscription the machine already has instead of asking for a
// credential it would then have to store. The cost is that this only works on
// a machine where one of them is installed, which for the audience of a local
// screenshot tool is not much of a cost.
//
// Everything here is optional. A project with no provider still writes copy;
// somebody types it.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// A Provider is one local agent CLI.
type Provider struct {
	ID    string
	Label string
	Bin   string
	// Note explains what the provider needs when it is not available.
	Note string
}

var Providers = []Provider{
	{ID: "claude", Label: "Claude Code", Bin: "claude", Note: "install the `claude` CLI and sign in"},
	{ID: "codex", Label: "Codex", Bin: "codex", Note: "install the `codex` CLI and sign in"},
}

// Available lists the providers actually on this machine. The editor uses it
// to decide whether to offer the button at all: an AI action that fails with
// "command not found" after the click is worse than one that was never there.
func Available() []Provider {
	var out []Provider
	for _, p := range Providers {
		if _, err := exec.LookPath(p.Bin); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func providerOf(id string) (Provider, bool) {
	for _, p := range Providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// A Runner is a configured provider. The zero Timeout means three minutes,
// which is generous for a paragraph of copy and short enough that a hung CLI
// does not hold an editor request open forever.
type Runner struct {
	Provider string
	// Model overrides the CLI's own default. Left empty for "whatever the CLI
	// is configured to use", which is almost always what is wanted.
	Model   string
	Timeout time.Duration
}

// Run sends one prompt and returns the agent's final message.
func (r Runner) Run(ctx context.Context, prompt string) (string, error) {
	p, ok := providerOf(r.Provider)
	if !ok {
		return "", fmt.Errorf("unknown provider %q", r.Provider)
	}
	if _, err := exec.LookPath(p.Bin); err != nil {
		return "", fmt.Errorf("%s is not installed: %s", p.Label, p.Note)
	}

	timeout := r.Timeout
	if timeout == 0 {
		timeout = 3 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Both CLIs are agents: they can read files and run commands, and they
	// take their bearings from the working directory. An empty one keeps a
	// copywriting request from wandering into whatever repository the app
	// happens to have been started from.
	dir, err := os.MkdirTemp("", "appeditions-ai-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)

	switch p.ID {
	case "claude":
		return r.runClaude(ctx, dir, prompt)
	case "codex":
		return r.runCodex(ctx, dir, prompt)
	}
	return "", fmt.Errorf("no runner for %q", p.ID)
}

// runClaude asks for the JSON envelope and reads `result` out of it. The
// envelope is what makes this reliable: the model's text arrives as a field
// rather than as whatever was on stdout, so a warning line printed by the CLI
// cannot end up inside the copy.
func (r Runner) runClaude(ctx context.Context, dir, prompt string) (string, error) {
	args := []string{"-p", "--output-format", "json"}
	if r.Model != "" {
		args = append(args, "--model", r.Model)
	}
	args = append(args, prompt)

	out, err := r.exec(ctx, dir, "claude", args...)
	if err != nil {
		return "", err
	}

	var envelope struct {
		Result  string `json:"result"`
		IsError bool   `json:"is_error"`
		Subtype string `json:"subtype"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return "", fmt.Errorf("claude returned something that is not its JSON envelope: %s", trim(string(out)))
	}
	if envelope.IsError {
		return "", fmt.Errorf("claude: %s: %s", envelope.Subtype, trim(envelope.Result))
	}
	return envelope.Result, nil
}

// runCodex writes the final message to a file rather than parsing stdout,
// which carries the whole session log.
func (r Runner) runCodex(ctx context.Context, dir, prompt string) (string, error) {
	out := filepath.Join(dir, "message.txt")
	args := []string{"exec", "--skip-git-repo-check", "-s", "read-only", "--output-last-message", out}
	if r.Model != "" {
		args = append(args, "-m", r.Model)
	}
	args = append(args, prompt)

	log, err := r.exec(ctx, dir, "codex", args...)
	if err != nil {
		// Codex reports a rejected model or an expired login on stdout and
		// exits 1. Passing that through verbatim is the difference between
		// "AI failed" and a message the author can act on.
		return "", fmt.Errorf("%w: %s", err, trim(lastErrorLine(string(log))))
	}
	data, readErr := os.ReadFile(out)
	if readErr != nil {
		return "", fmt.Errorf("codex wrote no message: %s", trim(lastErrorLine(string(log))))
	}
	return string(data), nil
}

func (r Runner) exec(ctx context.Context, dir, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return out, fmt.Errorf("%s timed out", bin)
	}
	if err != nil {
		return out, fmt.Errorf("%s failed", bin)
	}
	return out, nil
}

// lastErrorLine picks the most useful line out of a session log: the last one
// that looks like an error, or the last non-empty line.
func lastErrorLine(log string) string {
	lines := strings.Split(strings.TrimSpace(log), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "ERROR") || strings.Contains(lines[i], "error") {
			return lines[i]
		}
	}
	if len(lines) > 0 {
		return lines[len(lines)-1]
	}
	return ""
}

func trim(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}

// extractJSON pulls the first JSON value out of a model's reply. Asking for
// "only JSON" works most of the time; the rest of the time it arrives wrapped
// in a code fence or introduced by a sentence, and rejecting that would make
// the feature feel broken over a formatting preference.
func extractJSON(reply string) (string, error) {
	s := strings.TrimSpace(reply)
	if fence := strings.Index(s, "```"); fence >= 0 {
		rest := s[fence+3:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[nl+1:]
		}
		if end := strings.Index(rest, "```"); end >= 0 {
			s = strings.TrimSpace(rest[:end])
		}
	}
	start := strings.IndexAny(s, "[{")
	if start < 0 {
		return "", fmt.Errorf("no JSON in the reply: %s", trim(reply))
	}
	open, close := byte('['), byte(']')
	if s[start] == '{' {
		open, close = '{', '}'
	}
	depth, inString, escaped := 0, false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case escaped:
			escaped = false
		case c == '\\' && inString:
			escaped = true
		case c == '"':
			inString = !inString
		case inString:
		case c == open:
			depth++
		case c == close:
			depth--
			if depth == 0 {
				return s[start : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("the reply's JSON is not closed: %s", trim(reply))
}

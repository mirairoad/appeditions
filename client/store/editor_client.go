package store

import (
	"encoding/json"

	"github.com/mirairoad/howl-go/core/signal"

	"github.com/mirairoad/appeditions/internal/model"
)

// The browser's store, exposed reactively.
//
// On the server a store is per-process and read through the request context —
// two requests must never see each other's state. In the browser there is one
// user and one tab, so package-level signals are the right shape: any Mount,
// Unmount or handler can read them, and anything derived from them updates
// itself.
//
// This file is browser-only by construction. `howl check` reports server code
// that imports core/signal, which is the enforcement: a request handler setting
// a package-level signal is two requests writing one variable.
var editorClient = NewEditorStore()

// EditorClient is the browser-side store. Mutating it publishes to the signals
// below; the server's store is a different pointer and publishes nothing.
func EditorClient() *EditorStore { return editorClient }

var (
	// Project is the reactive document. It carries an explicit equality test
	// because a struct holding slices and maps is not comparable, and without
	// one every re-hydrate would wake every dependent even when nothing moved.
	Project = signal.WithEq(model.Project{}, sameProject)

	// Rev increments on every applied op. The preview tiles are PNGs the
	// server draws, so a local change cannot repaint them — it can only say
	// that the bytes behind an unchanged URL are now stale, which is what the
	// tile URL carries this for.
	Rev = signal.Of(0)

	// Screens is derived. DeriveEq means a change that leaves the set alone —
	// a colour, a tilt — does not wake anything that only reads the strip.
	Screens = signal.DeriveEq(func() int { return len(Project.Get().Screens()) })
)

// sameProject compares by encoded form. The document is a tree of pointers and
// maps: comparing those compares addresses, and a deep compare written by hand
// would be one more thing to keep in step with the model.
func sameProject(a, b model.Project) bool {
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(ja) == string(jb)
}

// publish mirrors a mutation into the signals. Only the browser's instance does
// this: the server's store is a different pointer, so concurrent requests never
// write these package-level variables.
func (s *EditorStore) publish() {
	if s != editorClient {
		return
	}
	sn := s.Snapshot()
	Project.Set(sn.Project)
	Rev.Set(sn.Rev)
}

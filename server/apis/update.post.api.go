package apis

import (
	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/internal/update"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// Update rebuilds this application from the tip and reinstalls it, by running
// the project's own update.sh.
//
// The script is the whole implementation and it is not reimplemented here: it
// compares the revision install.sh recorded against the tip, skips the work
// when they match, and otherwise clones, builds and installs. What this adds
// is a button in front of it.
//
// It takes no body. Reveal has to check the path it is given because that path
// comes from a request; this hands the shell a constant, so there is nothing
// to check and nothing to point somewhere else.
//
// Slow on purpose — a minute or so, building from source — and the request is
// held open for all of it, so the button stays busy and the answer is the
// script's own last line. Nothing here times a request out.
var Update = api.Define(api.Spec[api.None, api.None, apistore.Saved]{
	Name: "Update",
	Handler: func(r *api.Request[api.None, api.None]) (apistore.Saved, error) {
		said, err := update.Apply(r.Context())
		if err != nil {
			return apistore.Saved{}, api.BadRequest(err.Error())
		}
		// The new build is on disk; this process is still the old one. Saying
		// so is the difference between an update that looks like it did
		// nothing and one that is simply waiting for a relaunch.
		return apistore.Saved{
			ID:      update.Repo,
			Message: said + " — quit and reopen AppEditions to run it",
		}, nil
	},
})

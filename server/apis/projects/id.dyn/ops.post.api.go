package project

import (
	"github.com/mirairoad/howl-go/core/api"

	clientstore "github.com/mirairoad/appeditions/client/store"
	"github.com/mirairoad/appeditions/internal/model"
	"github.com/mirairoad/appeditions/server/apis/apistore"
)

// Ops applies one mutation.
//
// The body is the same [clientstore.Op] the browser applied locally a moment
// ago, and this runs the same [clientstore.ApplyTo] against the stored
// document. That is the whole point of the store package: the rule for "what
// does this control do to a project" exists once and runs in both places, so
// the optimistic update and the durable write cannot disagree.
//
// The reply is the snapshot the write produced. The browser reconciles to it
// rather than trusting its own copy — a local apply is a prediction, and this
// is the answer.
var Ops = api.Define(api.Spec[api.None, clientstore.Op, clientstore.EditorSnapshot]{
	Name: "ApplyOp",
	Handler: func(r *api.Request[api.None, clientstore.Op]) (clientstore.EditorSnapshot, error) {
		var failed error
		p, err := apistore.Get().Edit(r.Context(), r.Param("id"), func(p *model.Project) {
			// The closure may run more than once under the optimistic lock, so
			// the error is captured rather than returned: a retry re-applies
			// the op to the fresh document and overwrites this.
			failed = clientstore.ApplyTo(p, r.Body)
		})
		if err != nil {
			return clientstore.EditorSnapshot{}, apistore.Fail(err)
		}
		if failed != nil {
			return clientstore.EditorSnapshot{}, api.BadRequest(failed.Error())
		}
		return clientstore.EditorSnapshot{Project: p}, nil
	},
})

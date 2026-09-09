// Package apis is the endpoint tree. The file's location is its URL, the file
// name carries the method, and the Go types are the contract.
package apis

import (
	"os/exec"
	"runtime"
	"strings"

	"github.com/mirairoad/howl-go/core/api"

	"github.com/mirairoad/appeditions/server/apis/apistore"
)

type Target struct {
	Path string `json:"path"`
}

// Reveal opens a directory in the desktop's file manager.
//
// The path is checked against the data root before anything is executed. This
// endpoint is reachable only from a loopback socket in a webview, but it takes
// a path from a request body and hands it to the operating system, and "the
// only client is our own UI" is exactly the assumption that stops being true.
var Reveal = api.Define(api.Spec[api.None, Target, apistore.Saved]{
	Name: "Reveal",
	Handler: func(r *api.Request[api.None, Target]) (apistore.Saved, error) {
		root := apistore.Get().Root()
		if !strings.HasPrefix(r.Body.Path, root) {
			return apistore.Saved{}, api.Forbidden("that path is not inside the data directory")
		}

		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", r.Body.Path)
		case "linux":
			cmd = exec.Command("xdg-open", r.Body.Path)
		default:
			return apistore.Saved{}, api.BadRequest("no file manager wired up for " + runtime.GOOS)
		}
		if err := cmd.Start(); err != nil {
			return apistore.Saved{}, err
		}
		return apistore.Saved{ID: r.Body.Path}, nil
	},
})

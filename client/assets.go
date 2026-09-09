// Package client holds the browser-side assets.
//
// The embed lives here, one directory above public/, because go:embed refuses
// a `..` path and two binaries need the same files: the server and the desktop
// shell.
package client

import (
	"embed"
	"io/fs"
)

//go:embed public
var publicFS embed.FS

// Public is client/public rooted at its own directory, ready for
// app.Config.Public.
func Public() fs.FS {
	sub, err := fs.Sub(publicFS, "public")
	if err != nil {
		panic(err) // the path is a compile-time constant; this cannot fail
	}
	return sub
}

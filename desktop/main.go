// Command appeditions-desktop is AppEditions in a native window: WKWebView on macOS,
// WebKitGTK on Linux. No Electron and no bundled runtime — the window is a
// shim over a webview the operating system already ships, and the server is
// the same *app.App the CLI would otherwise Listen with, on a loopback socket
// nothing else on the machine can reach.
//
// Three ways to run it:
//
//	./appeditions-desktop                  its own loopback server, on a port only it knows
//	./appeditions-desktop -attach :9010    the `howl dev` front door
//	./appeditions-desktop -addr :9010      its own server, on a port you can open too
//
// The second is the development loop. `howl dev` restarts the server binary on
// every save but keeps its proxy port up, so the window connects once and
// reloads its content in place — nothing flashes and no window is killed.
//
// The third is for looking at the same session from a real browser while the
// window is open: with no -addr the listener is 127.0.0.1 on a kernel-picked
// port and the window is the only thing that ever learns the number, which is
// the right default and useless for comparing the two engines.
package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net"
	"strings"

	"github.com/mirairoad/howl-go/core/console"
	"github.com/mirairoad/howl-go/desktop"

	"github.com/mirairoad/appeditions/boot"
	"github.com/mirairoad/appeditions/internal/store"
)

func main() {
	attach := flag.String("attach", "", "point the window at an already-running server, e.g. :9010")
	addr := flag.String("addr", "", "serve on this address as well as opening the window, e.g. :9010")
	root := flag.String("data", store.DefaultRoot(), "where projects, screenshots and exports live")
	debug := flag.Bool("debug", false, "enable the webview inspector and debug logging")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	console.Setup(console.Options{Level: level})

	a, mux, s, err := boot.New(context.Background(), *root)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	// The webview binding assigns an autoreleased object to WKWebView's weak
	// UIDelegate property, so by the time a page loads there is none and
	// <input type="file"> does nothing at all. This installs one that is held
	// strongly. Queued here and applied once Run starts the event loop.
	installFilePanel()

	// Same shape, same reason: a WKWebView with no download delegate ignores
	// an attachment response entirely, so "Save a copy" would do nothing.
	installDownloads()

	// Same shape again: since macOS 13.3 developerExtrasEnabled alone leaves
	// the inspector unattachable.
	if *debug {
		enableInspector()
	}

	// -addr serves the window's own session on a port a browser can open, by
	// listening here and pointing the window at it through the Attach path.
	// Bound to loopback explicitly: a bare ":9010" is every interface on the
	// machine, and this server has no authentication because nothing was ever
	// supposed to reach it but the window.
	if *attach == "" && *addr != "" {
		ln, err := net.Listen("tcp", loopback(*addr))
		if err != nil {
			log.Fatal(err)
		}
		go a.Serve(ln, mux) //nolint:errcheck // the window closing is the exit path
		slog.Info("serving the window's session", "url", "http://"+ln.Addr().String())
		*attach = ln.Addr().String()
	}

	// Not log.Fatal(desktop.Run(...)): closing the window is how the user
	// quits, so it returns nil there, and the a.Listen idiom would print
	// "<nil>" and exit 1.
	if err := desktop.Run(a, mux, desktop.Options{
		Title:  "AppEditions",
		Width:  1440,
		Height: 940,
		Attach: *attach,
		Debug:  *debug,
	}); err != nil {
		log.Fatal(err)
	}
}

// loopback pins a bare port to 127.0.0.1. ":9010" means every interface, which
// for a server with no authentication in front of it means every machine on
// the network that can reach this one.
func loopback(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

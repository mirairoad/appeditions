//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit
void appkit_enable_inspector(void);
*/
import "C"

import "log/slog"

// enableInspector makes the window's WKWebView attachable from Safari's
// Develop menu, and makes right-click → Inspect Element open something.
//
// webview.New(debug) only sets developerExtrasEnabled, which stopped being
// sufficient in macOS 13.3; see filepanel_darwin.m. Called before desktop.Run
// when -debug is set, and applied as soon as the run loop starts.
func enableInspector() { C.appkit_enable_inspector() }

// appkitInspectorEnabled is called from the Objective-C side once it has found
// the WKWebView, or given up. It logs for the same reason the file panel does:
// an inspector that never attaches is indistinguishable from one nobody opened.
//
//export appkitInspectorEnabled
func appkitInspectorEnabled(ok C.int) {
	if ok != 0 {
		slog.Info("web inspector enabled — Safari ▸ Develop ▸ this Mac ▸ AppEditions")
		return
	}
	slog.Warn("no WKWebView found to enable the inspector on; -debug will show no devtools")
}

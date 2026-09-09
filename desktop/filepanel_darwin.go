//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit
void appkit_install_file_panel(void);
*/
import "C"

import "log/slog"

// installFilePanel gives the window a working file picker.
//
// It is a workaround for a lifetime bug in the pinned webview binding, which
// assigns an autoreleased object to WKWebView's weak UIDelegate property; see
// filepanel_darwin.m. Called before desktop.Run, and applied as soon as the
// run loop starts.
func installFilePanel() { C.appkit_install_file_panel() }

// appkitPanelInstalled is called from the Objective-C side once it has found
// the WKWebView, or given up.
//
// It logs, because the failure it guards against is invisible: a picker that
// opens nothing looks identical to a picker nobody clicked, and the first
// version of this shipped with the install call missing entirely and no way to
// tell from the outside.
//
//export appkitPanelInstalled
func appkitPanelInstalled(ok C.int) {
	if ok != 0 {
		slog.Debug("file panel installed")
		return
	}
	slog.Warn("no WKWebView found to install the file panel on; choosing files will do nothing")
}

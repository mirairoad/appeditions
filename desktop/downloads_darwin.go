//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit
void appkit_install_downloads(void);
*/
import "C"

import "log/slog"

// installDownloads makes a Content-Disposition: attachment response actually
// produce a file.
//
// Without a WKDownloadDelegate the window does nothing with one — no panel, no
// file, no console message — which is indistinguishable from a button nobody
// pressed. See filepanel_darwin.m; it is the same shape as the file picker.
func installDownloads() { C.appkit_install_downloads() }

//export appkitDownloadsInstalled
func appkitDownloadsInstalled(ok C.int) {
	if ok != 0 {
		slog.Debug("downloads wired to ~/Downloads")
		return
	}
	slog.Warn("no WKWebView found to handle downloads on; Save a copy will do nothing")
}

//export appkitDownloadFinished
func appkitDownloadFinished(ok C.int) {
	if ok != 0 {
		slog.Info("saved a copy to ~/Downloads")
		return
	}
	slog.Warn("a download failed")
}

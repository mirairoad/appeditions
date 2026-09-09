//go:build !darwin

package main

// enableInspector is a no-op away from macOS: WebKitGTK's inspector is already
// on whenever webview is built with debug.
func enableInspector() {}

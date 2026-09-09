//go:build !darwin

package main

// installDownloads is a no-op away from macOS: WebKitGTK downloads through its
// own machinery.
func installDownloads() {}

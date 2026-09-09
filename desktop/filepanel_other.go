//go:build !darwin

package main

// installFilePanel is a no-op away from macOS: WebKitGTK runs its own file
// chooser and needs nothing from us.
func installFilePanel() {}

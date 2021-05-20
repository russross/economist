// +build darwin,!linux,!windows

package main

import (
	"path/filepath"
	"syscall"
)

var Target = filepath.Join(string(filepath.Separator), "volumes", "ECONOMIST", "ec")
var SkipSections = map[string]bool{
	"The_Americas":           true,
	"Asia":                   true,
	"China":                  true,
	"Middle_East_and_Africa": true,
	"Europe":                 true,
}

func sync() {
	syscall.Sync()
}

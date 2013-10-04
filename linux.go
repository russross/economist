// +build linux,!windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
)

var Target = filepath.Join(string(filepath.Separator), "media", os.Getenv("USER"), "economist", "ec")
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

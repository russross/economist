// +build linux,!windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
)

var Target = filepath.Join(string(filepath.Separator), "media", os.Getenv("USER"), "ECONOMIST", "ec")
var SkipSections = map[string]bool{
	"Letters":                true,
	"The_Americas":           true,
	"Asia":                   true,
	"China":                  true,
	"Middle_East_and_Africa": true,
	"Europe":                 true,
	"Finance_and_economics":  true,
	"Books_and_arts":         true,
	"Graphic_detail":         true,
}

func sync() {
	syscall.Sync()
}

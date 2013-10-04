// +build !linux,windows

package main

import "path/filepath"

var Target = filepath.Join(string(filepath.Separator), "economist", "ec")
var SkipSections = map[string]bool{}

func sync() {
	// do nothing
}

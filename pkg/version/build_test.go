// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0
// https://www.apache.org/licenses/LICENSE-2.0

package version

import (
	"os"
	"regexp"
	"testing"
)

// bump_build.sh finds the number by the exact form of its line: this is the line it looks for, so
// that a reformatted declaration does not leave the release with a number that never goes up.
func TestBuildLineIsTheFormTheBumpScriptReads(t *testing.T) {
	src, err := os.ReadFile("build.go")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(regexp.MustCompile(`(?m)^const Build uint64 = [0-9]+$`).FindAll(src, -1)); n != 1 {
		t.Errorf("build.go has %d lines of the form 'const Build uint64 = N', want 1", n)
	}
}

package zenzx

import (
	"fmt"
	"strings"
	"testing"
)

// -version and About say the version and the build: the number that tells two binaries of one version
// apart.
func TestTheVersionLineSaysTheBuild(t *testing.T) {
	got := versionLine()
	want := fmt.Sprintf("%s (build %d)", version, buildNumber)
	if got != want {
		t.Errorf("the version line is %q, want %q", got, want)
	}
	if !strings.Contains(got, "build ") {
		t.Errorf("the version line %q does not name the build", got)
	}
	// the manual's file name is the version's alone: a build does not rename it
	if strings.Contains(manualName(version), "build") {
		t.Errorf("the manual is named %q", manualName(version))
	}
}

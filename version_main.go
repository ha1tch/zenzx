package zenzx

import (
	"fmt"

	zenzxversion "github.com/ha1tch/zenzx/pkg/version"
)

// version is the build-time version string. It defaults to the canonical
// constant in pkg/version and can be overridden at build time with:
//
//	-ldflags "-X github.com/ha1tch/zenzx.version=$(git describe --tags --always --dirty)"
//
// The build scripts pass the git-derived value; a plain `go build` falls back
// to the pkg/version constant.
var version = zenzxversion.Version

// buildNumber is the number of the build, which goes up by one for each release that is built and
// never goes back (pkg/version/build.go): two binaries of one version are told apart by it.
var buildNumber = zenzxversion.Build

// versionLine is the version and the build, as -version and About say them.
func versionLine() string {
	return fmt.Sprintf("%s (build %d)", version, buildNumber)
}

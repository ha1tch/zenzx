package main

import zenzxversion "github.com/ha1tch/zenzx/pkg/version"

// version is the build-time version string. It defaults to the canonical
// constant in pkg/version and can be overridden at build time with:
//   -ldflags "-X main.version=$(git describe --tags --always --dirty)"
//
// The build scripts pass the git-derived value; a plain `go build` falls back
// to the pkg/version constant.
var version = zenzxversion.Version

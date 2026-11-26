// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0
// https://www.apache.org/licenses/LICENSE-2.0

package version

// Build is the number of the build: an unsigned integer that goes up by one each time a release
// is built (bump_build.sh, the "build-number" step of the release) and is never reset, not by a
// new version and not by a re-release of the same one. Two binaries that say the same version
// are told apart by it. 0 is a tree that was never released: a build made from a checkout between
// releases.
const Build uint64 = 17

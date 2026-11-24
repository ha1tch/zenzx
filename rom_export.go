// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

// ResolveROMBytes returns the bytes of a named ROM, looking in romDir on disk
// first and falling back to the copy embedded in the binary. It is the
// exported form of the package's own ROM resolution, for embedders (such as
// the zendbg CLI) that build a machine and need its ROM without reaching into
// the package internals.
func ResolveROMBytes(name, romDir string) ([]byte, error) {
	return resolveROMBytes(name, romDir)
}

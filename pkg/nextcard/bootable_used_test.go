// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package nextcard

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The card the system is copied from is usually one that has been in use, whose free space
// record no longer says what the FAT does. A Next starts from such a card, so it can be the
// source; what is made from it is held to the full check all the same.
func TestMakeBootableFromACardThatHasBeenInUse(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "next-card.img")
	systemCard(t, source)

	f, err := os.OpenFile(source, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	vol, err := Open(f)
	if err != nil {
		t.Fatal(err)
	}
	record := (vol.geo.start+int64(vol.geo.fsInfo))*sectorSize + 488
	b := make([]byte, 1)
	if _, err := f.ReadAt(b, record); err != nil {
		t.Fatal(err)
	}
	b[0]++
	if _, err := f.WriteAt(b, record); err != nil {
		t.Fatal(err)
	}
	if err := VerifyCard(f); err == nil {
		t.Fatal("the card still passes VerifyCard; the test has not made it a used one")
	}
	f.Close()

	target := filepath.Join(dir, "minimal.img")
	root := NewDir("")
	root.AddDir("games").Add("mine.bas", []byte("10 PRINT 1"))
	if _, err := NewCard(target, SmallestCard, root); err != nil {
		t.Fatal(err)
	}
	if _, err := MakeBootable(context.Background(), source, target, nil); err != nil {
		t.Fatalf("a card that has been in use was turned away as the source: %v", err)
	}
	g, err := os.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	if err := VerifyCard(g); err != nil {
		t.Fatalf("the card made from it is not bootable: %v", err)
	}
}

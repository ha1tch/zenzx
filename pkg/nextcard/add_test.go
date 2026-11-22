package nextcard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAddFilesKeepsTheCardAndAddsToIt(t *testing.T) {
	img, _, files := buildTestImage(t)
	card := filepath.Join(t.TempDir(), CardName)
	if err := os.WriteFile(card, img, 0o644); err != nil {
		t.Fatal(err)
	}
	add := []File{{"gallop.nex", bytes.Repeat([]byte("horse"), 5000)}, {"fuji.nex", []byte("mountain")}}
	if err := AddFiles(context.Background(), card, "/zenzx", add, nil); err != nil {
		t.Fatal(err)
	}
	// Again, with one replaced and one into a folder that is already there and nested.
	again := []File{{"FUJI.NEX", []byte("a second mountain")}}
	if err := AddFiles(context.Background(), card, "zenzx", again, nil); err != nil {
		t.Fatal(err)
	}
	if err := AddFiles(context.Background(), card, "/demos/A long folder name/more", []File{{"x.bin", []byte{1, 2, 3}}}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(card + ".part"); err == nil {
		t.Error("the working file was left behind")
	}
	data, err := os.ReadFile(card)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len(img) {
		t.Fatalf("the card is %d bytes, was %d", len(data), len(img))
	}
	vol, err := Open(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]byte{
		"/zenzx/gallop.nex":                    add[0].Data,
		"/zenzx/fuji.nex":                      again[0].Data,
		"/demos/A long folder name/more/x.bin": {1, 2, 3},
	}
	for name, content := range files {
		want[name] = content
	}
	for name, content := range want {
		n := vol.Root.Find(name)
		if n == nil {
			t.Errorf("%s is not on the card", name)
			continue
		}
		got, err := vol.ReadFile(n)
		if err != nil {
			t.Errorf("%s: %v", name, err)
		} else if !bytes.Equal(got, content) {
			t.Errorf("%s: %d bytes, want %d", name, len(got), len(content))
		}
	}
	count := 0
	vol.Root.Find("zenzx").Walk(func(_ string, n *Node) {
		if !n.IsDir() {
			count++
		}
	})
	if count != 2 {
		t.Errorf("/zenzx holds %d files, want 2 (the replaced one once)", count)
	}
}

func TestAddFilesRefusesWhatCannotBe(t *testing.T) {
	img, _, _ := buildTestImage(t)
	card := filepath.Join(t.TempDir(), CardName)
	if err := os.WriteFile(card, img, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		folder string
		file   File
	}{
		{"/README.md", File{"a.nex", nil}}, // a file where the folder would go
		{"/", File{"demos", nil}},          // a folder where the file would go
		{"/zenzx", File{"sub/a.nex", nil}}, // a name with a path in it
		{"/zenzx", File{"", nil}},
	} {
		if err := AddFiles(context.Background(), card, c.folder, []File{c.file}, nil); err == nil {
			t.Errorf("AddFiles(%q, %q) was accepted", c.folder, c.file.Name)
		}
	}
	after, _ := os.ReadFile(card)
	if !bytes.Equal(after, img) {
		t.Error("a refused call changed the card")
	}
}

func TestNewCardIsACardThatCanBeAddedTo(t *testing.T) {
	card := filepath.Join(t.TempDir(), "examples.img")
	root := NewDir("")
	root.AddDir("zenzx").Add("gallop.nex", []byte("horse"))
	lay, err := NewCard(card, SmallestCard, root)
	if err != nil {
		t.Fatal(err)
	}
	if lay.Clusters < MinFAT32Clusters || lay.Files != 1 {
		t.Fatalf("layout %+v", lay)
	}
	if _, err := NewCard(card, SmallestCard, root); err == nil {
		t.Error("a card that was already there was written over")
	}
	if _, err := NewCard(card+"2", 1<<20, root); err == nil {
		t.Error("a card too small for FAT32 was made")
	}
	if err := AddFiles(context.Background(), card, "/zenzx", []File{{"fuji.nex", []byte("mountain")}}, nil); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(card)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	vol, err := Open(f)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"/zenzx/gallop.nex": "horse", "/zenzx/fuji.nex": "mountain"} {
		n := vol.Root.Find(name)
		if n == nil {
			t.Fatalf("%s is not on the card", name)
		}
		if got, _ := vol.ReadFile(n); string(got) != want {
			t.Errorf("%s holds %q", name, got)
		}
	}
}

func TestNewCardEmptyAtBothSizesTheMenuOffers(t *testing.T) {
	for _, size := range []int64{SmallestCard, 1 << 30} {
		card := filepath.Join(t.TempDir(), "empty.img")
		lay, err := NewCard(card, size, NewDir(""))
		if err != nil {
			t.Fatalf("%d bytes: %v", size, err)
		}
		st, err := os.Stat(card)
		if err != nil || st.Size() != size {
			t.Fatalf("%d bytes: the file is %v", size, st)
		}
		f, err := os.Open(card)
		if err != nil {
			t.Fatal(err)
		}
		vol, err := Open(f)
		f.Close()
		if err != nil {
			t.Fatalf("%d bytes: %v", size, err)
		}
		if len(vol.Root.Children) != 0 || lay.Files != 0 || vol.ClusterCount() < MinFAT32Clusters {
			t.Errorf("%d bytes: %d entries, %d clusters", size, len(vol.Root.Children), vol.ClusterCount())
		}
	}
}

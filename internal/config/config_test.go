package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestListSavesOrdersByRecencyDescending(t *testing.T) {
	dir := t.TempDir()

	write := func(name string, modTime time.Time) {
		path := filepath.Join(dir, name+saveExt)
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
	}

	now := time.Now()
	write("alpha", now.Add(-2*time.Hour))
	write("charlie", now)
	write("bravo", now.Add(-1*time.Hour))
	// A non-save file should be ignored.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}

	saves, err := ListSaves(dir)
	if err != nil {
		t.Fatalf("ListSaves: %v", err)
	}

	want := []string{"charlie", "bravo", "alpha"}
	if !reflect.DeepEqual(saves, want) {
		t.Fatalf("expected %v, got %v", want, saves)
	}
}

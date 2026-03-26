package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSessionStoreAliases(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	if err := store.PutAlias("codex", "api", "/Users/kimtaeyun/api"); err != nil {
		t.Fatalf("put alias: %v", err)
	}
	if err := store.PutAlias("codex", "bridge", "/Users/kimtaeyun/bridge"); err != nil {
		t.Fatalf("put alias: %v", err)
	}

	got := store.ListAliases("codex")
	want := map[string]string{
		"api":    "/Users/kimtaeyun/api",
		"bridge": "/Users/kimtaeyun/bridge",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListAliases mismatch\nwant: %#v\ngot:  %#v", want, got)
	}

	if err := store.DeleteAlias("codex", "api"); err != nil {
		t.Fatalf("delete alias: %v", err)
	}
	got = store.ListAliases("codex")
	want = map[string]string{
		"bridge": "/Users/kimtaeyun/bridge",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListAliases after delete mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestSessionStoreRecentDirs(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	for i := 1; i <= cwdRecentLimit+2; i++ {
		if err := store.PutRecentDir("codex", filepath.Join("/Users/kimtaeyun", "repo", string(rune('a'+i)))); err != nil {
			t.Fatalf("put recent dir %d: %v", i, err)
		}
	}
	if err := store.PutRecentDir("codex", "/Users/kimtaeyun/repo/c"); err != nil {
		t.Fatalf("put duplicate recent dir: %v", err)
	}

	got := store.ListRecentDirs("codex")
	if len(got) != cwdRecentLimit {
		t.Fatalf("expected %d recent dirs, got %d", cwdRecentLimit, len(got))
	}
	if got[0] != "/Users/kimtaeyun/repo/c" {
		t.Fatalf("expected duplicate to move to front, got %q", got[0])
	}
}

package source

import (
	"context"
	"strings"
	"testing"
)

func TestCuratedName(t *testing.T) {
	if NewCurated().Name() != "Open Media" {
		t.Errorf("Name = %q, want Open Media", NewCurated().Name())
	}
}

func TestCuratedEmptyQueryReturnsAll(t *testing.T) {
	all, err := NewCurated().Search(context.Background(), "")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(all) == 0 {
		t.Fatal("empty query should return the whole curated catalog")
	}
	for _, r := range all {
		if r.Source != "Open Media" {
			t.Errorf("result %q has Source %q, want Open Media", r.Title, r.Source)
		}
		if !strings.HasPrefix(r.Magnet, "magnet:?") || !strings.Contains(r.Magnet, "xt=urn%3Abtih%3A") {
			t.Errorf("result %q magnet looks malformed: %q", r.Title, r.Magnet)
		}
		if r.TorrentURL != "" {
			t.Errorf("curated items are magnet-only; %q has TorrentURL %q", r.Title, r.TorrentURL)
		}
	}
}

func TestCuratedFiltersByQuery(t *testing.T) {
	got, err := NewCurated().Search(context.Background(), "bunny")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || !strings.Contains(strings.ToLower(got[0].Title), "bunny") {
		t.Fatalf("query 'bunny' = %v, want exactly Big Buck Bunny", titlesOf(got))
	}

	// Matching is case-insensitive and spans keywords (e.g. "blender").
	blender, _ := NewCurated().Search(context.Background(), "BLENDER")
	if len(blender) < 2 {
		t.Errorf("query 'BLENDER' returned %d, want several Blender open movies", len(blender))
	}

	none, _ := NewCurated().Search(context.Background(), "zzz-no-such-title")
	if len(none) != 0 {
		t.Errorf("no-match query returned %d results, want 0", len(none))
	}
}

func titlesOf(rs []Result) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Title
	}
	return out
}

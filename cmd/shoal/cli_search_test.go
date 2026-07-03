package main

import (
	"testing"

	"github.com/StrangeNoob/shoal/internal/source"
)

func TestParseSearchArgsAllowsFlagsAfterQuery(t *testing.T) {
	opts, query, err := parseSearchArgs([]string{"ubuntu iso", "--json", "--limit", "2", "--category=linux"})
	if err != nil {
		t.Fatalf("parseSearchArgs: %v", err)
	}
	if query != "ubuntu iso" {
		t.Fatalf("query = %q, want ubuntu iso", query)
	}
	if !opts.json || opts.limit != 2 || opts.category != "linux" {
		t.Fatalf("opts = %+v", opts)
	}
}

func TestParseDownloadArgsAllowsFlagsAfterTarget(t *testing.T) {
	opts, target, err := parseDownloadArgs([]string{"magnet:?xt=urn:btih:abc", "--json", "--open", "--index", "3", "--timeout=5m"})
	if err != nil {
		t.Fatalf("parseDownloadArgs: %v", err)
	}
	if target != "magnet:?xt=urn:btih:abc" {
		t.Fatalf("target = %q", target)
	}
	if !opts.json || !opts.open || opts.index != 3 || opts.timeout.String() != "5m0s" {
		t.Fatalf("opts = %+v", opts)
	}
}

func TestFilterCategoryUsesCaseInsensitiveSubstring(t *testing.T) {
	results := filterCategory([]source.Result{
		{Category: "TV Shows", Title: "match"},
		{Category: "movies", Title: "miss"},
	}, "shows")
	if len(results) != 1 || results[0].Title != "match" {
		t.Fatalf("filtered = %+v", results)
	}
}

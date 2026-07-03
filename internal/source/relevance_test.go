package source

import "testing"

func TestRelevanceOrdersByMatchQuality(t *testing.T) {
	// A higher score must mean a closer title. Assert relative ordering rather
	// than exact values so the scoring weights can be tuned without churn.
	q := "jimmy fallon sydney sweeney"
	exact := Relevance(q, "Jimmy Fallon 2025 06 05 Sydney Sweeney 480p x264-mSD")
	partial := Relevance(q, "Saturday Night Live S49E13 Sydney Sweeney 1080p WEB h264-EDITH")
	unrelated := Relevance(q, "Ubuntu 24.04 Desktop amd64")

	if !(exact > partial) {
		t.Errorf("full match (%.3f) should beat partial match (%.3f)", exact, partial)
	}
	if !(partial > unrelated) {
		t.Errorf("partial match (%.3f) should beat unrelated (%.3f)", partial, unrelated)
	}
	if unrelated != 0 {
		t.Errorf("unrelated title should score 0, got %.3f", unrelated)
	}
}

func TestRelevanceExactAndEmpty(t *testing.T) {
	if got := Relevance("Ubuntu 24.04", "ubuntu.24.04"); got != 1 {
		t.Errorf("token-identical title should score 1.0, got %.3f", got)
	}
	if got := Relevance("", "anything"); got != 0 {
		t.Errorf("empty query should score 0, got %.3f", got)
	}
	if got := Relevance("anything", ""); got != 0 {
		t.Errorf("empty title should score 0, got %.3f", got)
	}
}

func TestRelevancePhraseBeatsScattered(t *testing.T) {
	q := "black mirror"
	phrase := Relevance(q, "Black Mirror S06E01 1080p")
	scattered := Relevance(q, "Mirror Cleaning Guide featuring a black cloth")
	if !(phrase > scattered) {
		t.Errorf("contiguous phrase (%.3f) should beat scattered words (%.3f)", phrase, scattered)
	}
}

func TestRankByRelevanceStampsAndOrders(t *testing.T) {
	rs := []Result{
		{Title: "Saturday Night Live Sydney Sweeney 1080p", Seeders: 500},
		{Title: "Jimmy Fallon Sydney Sweeney 480p", Seeders: 5},
		{Title: "Jimmy Fallon Sydney Sweeney 1080p", Seeders: 50},
	}
	RankByRelevance(rs, "jimmy fallon sydney sweeney")

	// Both Jimmy Fallon titles match better than SNL and must lead, despite SNL
	// having by far the most seeders — relevance outranks health across bands.
	if rs[0].Title[:12] != "Jimmy Fallon" || rs[1].Title[:12] != "Jimmy Fallon" {
		t.Fatalf("best matches should lead, got order: %q, %q, %q", rs[0].Title, rs[1].Title, rs[2].Title)
	}
	// Within the same relevance band, the healthier swarm wins (1080p: 50 > 480p: 5).
	if rs[0].Seeders != 50 {
		t.Fatalf("within-band tie should prefer more seeders, got %d first", rs[0].Seeders)
	}
	// Relevance is stamped onto every result.
	for _, r := range rs {
		if r.Relevance <= 0 {
			t.Errorf("result %q left unscored (Relevance=%.3f)", r.Title, r.Relevance)
		}
	}
}

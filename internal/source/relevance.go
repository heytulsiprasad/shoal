package source

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Relevance scores how well title answers query, in [0,1]. It is the "best
// match" signal: providers return whatever their own search engine thought was
// relevant (often loosely), and those hit-lists are merged across sites with
// wildly different quality — so shoal re-scores every title against the actual
// query text to float the closest matches to the top.
//
// The score blends three lexical signals, each in [0,1]:
//
//	coverage  — what share of the query's words appear in the title
//	phrase    — the longest run of query words that appear consecutively,
//	            as a share of the query (rewards "jimmy fallon" over a title
//	            that merely happens to contain both words far apart)
//	compact   — how much of the title is query (a tight title beats one
//	            padded with release tags, dates and codecs)
//
// coverage dominates; an exact token match short-circuits to 1.0.
func Relevance(query, title string) float64 {
	q := normalizeTokens(query)
	t := normalizeTokens(title)
	if len(q) == 0 || len(t) == 0 {
		return 0
	}
	if equalTokens(q, t) {
		return 1
	}

	present := make(map[string]bool, len(t))
	for _, tk := range t {
		present[tk] = true
	}

	var matched float64
	for _, qi := range q {
		matched += tokenScore(qi, t, present)
	}
	coverage := matched / float64(len(q))
	if coverage == 0 {
		return 0 // no query word appears at all — irrelevant, not merely compact
	}
	phrase := float64(longestRun(q, t)) / float64(len(q))
	compact := math.Min(1, float64(len(q))/float64(len(t)))

	score := 0.6*coverage + 0.3*phrase + 0.1*compact
	// Reserve 1.0 for an exact token match (handled above) so ranking can treat
	// "perfect" as a distinct tier.
	return math.Min(score, 0.999)
}

// tokenScore is how well a single query token qi is covered by the title
// tokens: 1 for an exact token, 0.7 for a title token qi is a prefix of, 0.5
// for a mere substring, 0 otherwise. Short tokens require a stronger match to
// avoid noise (a stray "s" matching every title).
func tokenScore(qi string, t []string, present map[string]bool) float64 {
	if present[qi] {
		return 1
	}
	best := 0.0
	for _, tk := range t {
		switch {
		case len(qi) >= 2 && strings.HasPrefix(tk, qi):
			if 0.7 > best {
				best = 0.7
			}
		case len(qi) >= 3 && strings.Contains(tk, qi):
			if 0.5 > best {
				best = 0.5
			}
		}
	}
	return best
}

// longestRun is the length of the longest sequence of query tokens (in query
// order) that appears contiguously in the title. O(len(q)·len(t)) — trivial for
// torrent titles.
func longestRun(q, t []string) int {
	best := 0
	for i := range q {
		for j := range t {
			k := 0
			for i+k < len(q) && j+k < len(t) && q[i+k] == t[j+k] {
				k++
			}
			if k > best {
				best = k
			}
		}
	}
	return best
}

// normalizeTokens lowercases s and splits it into alphanumeric tokens, so
// "Jimmy.Fallon.2025-06-05" and "jimmy fallon 2025 06 05" tokenize identically.
func normalizeTokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func equalTokens(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// relevanceBucket quantizes a score into 0.05-wide bands. Ranking compares
// buckets rather than raw scores so that two near-identical titles (e.g. the
// same episode in 720p vs 1080p) are treated as an equal-relevance tie and
// ordered by swarm health instead of by an insignificant lexical difference.
func relevanceBucket(score float64) int {
	return int(math.Round(score / 0.05))
}

// RankByRelevance stamps each result's Relevance against query and orders them
// best-match first. Within a relevance band it prefers the healthier swarm
// (more seeders, then higher popularity), so the top result is the closest
// title you can actually download quickly. Stable, so equal results keep their
// incoming (provider) order.
func RankByRelevance(results []Result, query string) {
	for i := range results {
		results[i].Relevance = Relevance(query, results[i].Title)
	}
	sort.SliceStable(results, func(a, b int) bool {
		return RelevanceLess(results[a], results[b])
	})
}

// RelevanceLess reports whether a should rank before b under best-match
// ordering: higher relevance band first, then more seeders, then higher
// popularity. Shared with the UI so its live sort matches RankByRelevance.
func RelevanceLess(a, b Result) bool {
	if ba, bb := relevanceBucket(a.Relevance), relevanceBucket(b.Relevance); ba != bb {
		return ba > bb
	}
	if a.Seeders != b.Seeders {
		return a.Seeders > b.Seeders
	}
	return a.Popularity > b.Popularity
}

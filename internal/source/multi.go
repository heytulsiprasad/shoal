package source

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// MultiSource fans a single query out across several providers concurrently and
// returns their merged results. It implements Source itself, so the rest of the
// app (the UI holds one source.Source) needs no changes to gain multi-source
// search — each Result still carries its own .Source label for provenance.
type MultiSource struct {
	sources []Source
}

// NewMulti composes several sources into one. Order is irrelevant; results are
// merged and re-sorted.
func NewMulti(sources ...Source) *MultiSource {
	return &MultiSource{sources: sources}
}

// Name is the providing source's name when there's one, else a count (kept short
// for the sidebar; per-result provenance lives on Result.Source).
func (m *MultiSource) Name() string {
	switch len(m.sources) {
	case 0:
		return "no sources"
	case 1:
		return m.sources[0].Name()
	default:
		return fmt.Sprintf("%d sources", len(m.sources))
	}
}

// Search queries every source concurrently and merges the hits. A source that
// fails is skipped; an error is only returned if *every* source failed, so one
// flaky provider never blanks the results.
func (m *MultiSource) Search(ctx context.Context, query string) ([]Result, error) {
	results := make([][]Result, len(m.sources))
	errs := make([]error, len(m.sources))

	var wg sync.WaitGroup
	for i, s := range m.sources {
		wg.Add(1)
		go func(i int, s Source) {
			defer wg.Done()
			results[i], errs[i] = s.Search(ctx, query) // each goroutine owns its own index — no shared writes
		}(i, s)
	}
	wg.Wait()

	var merged []Result
	var firstErr error
	failed := 0
	for i := range m.sources {
		if errs[i] != nil {
			failed++
			if firstErr == nil {
				firstErr = errs[i]
			}
			continue
		}
		merged = append(merged, results[i]...)
	}
	if len(m.sources) > 0 && failed == len(m.sources) {
		return nil, firstErr
	}

	// Popularity scales differ between providers, so this is a rough health
	// ordering, not an exact ranking. Stable so same-popularity ties keep their
	// source order.
	sort.SliceStable(merged, func(a, b int) bool { return merged[a].Popularity > merged[b].Popularity })
	return merged, nil
}

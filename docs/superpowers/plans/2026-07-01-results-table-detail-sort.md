# Search Results Table + Detail Screen + Live Progress + Sorting — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace shoal's two-line search-results list with a bordered, sortable, columnar table; add a torrent detail screen; and stream live per-source search progress — keeping the existing sidebar/header chrome.

**Architecture:** Extend `source.Result` with data the scrapers already parse but discard. Add a streaming `MultiSource.SearchStream` that emits one update per source. Wire the Bubble Tea model to pump those updates (with a generation guard against stale searches), add a modal sort picker, and a detail sub-screen. Rendering gets one reusable `titledBox` helper plus a column table and detail layout.

**Tech Stack:** Go 1.26, Bubble Tea v1.3.10, Lipgloss v1.1.0, `github.com/atotto/clipboard` (already in the module graph).

## Global Constraints

- TDD: write the failing test first for every behavior (project rule).
- No Claude attribution in commits (no `Co-Authored-By` / "Generated with" trailer).
- The working directory is **not** a git repo. Treat each "Checkpoint" step as the task gate: the named `go test` command must pass. If you `git init`, commits are optional and must omit the attribution trailer.
- Follow existing `internal/ui` patterns: value-receiver `Model`, package-global `st`/`activePalette` styles, `truncate`/`max`/`min` helpers already exist.
- Reuse existing lipgloss styles (`st.Accent`, `st.Faint`, `st.Meta`, `st.Row`, `st.RowSel`, `st.Key`, `st.KeyDesc`, `st.Footer`, `st.FooterSep`, `st.SectionHead`) — do **not** modify `theme.go`.
- Sources with no seed/leech data render `—`; unknown detail rows are omitted. Ratio = `Leechers ÷ Seeders`, `Seeders == 0 → +Inf`.

---

### Task 1: Extend `source.Result` and populate scrapers

**Files:**
- Modify: `internal/source/source.go` (add fields)
- Modify: `internal/source/piratebay.go`, `yts.go`, `eztv.go`, `solidtorrents.go`, `nyaa.go`, `x1337.go`, `subsplease.go`, `torlink_helpers.go` (populate; delete `_ =` residue)
- Test: `internal/source/torlink_test.go` (extend assertions)

**Interfaces:**
- Produces: `source.Result` gains `Seeders int64`, `Leechers int64`, `Files int`, `Added int64` (unix seconds; `0` when unknown). Consumed by Tasks 3, 5, 8, 9.

- [ ] **Step 1: Extend existing test assertions (failing).**

In `internal/source/torlink_test.go`, tighten these assertions:

`TestPirateBaySearchFiltersCategory` — replace the final `if` with:
```go
	g := got[0]
	if len(got) != 1 || g.Title != "Movie" || g.Category != "movies" || g.Popularity != 10 {
		t.Fatalf("results = %+v, want only Movie", got)
	}
	if g.Seeders != 10 || g.Leechers != 1 || g.Files != 2 || g.Added != 1710000000 {
		t.Fatalf("piratebay fields = %+v, want seeders 10 leechers 1 files 2 added 1710000000", g)
	}
```

`TestYTSSearchMapsMovieTorrents` — after the existing title check add:
```go
	if got[0].Seeders != 9 || got[0].Leechers != 2 || got[0].Added != 1710000000 {
		t.Fatalf("yts fields = %+v, want seeders 9 leechers 2 added 1710000000", got[0])
	}
```

`TestEZTVSearchOnlyBrowsesLatest` — after the final `if` add:
```go
	if got[0].Seeders != 7 || got[0].Leechers != 3 || got[0].Added != 1710000000 {
		t.Fatalf("eztv fields = %+v, want seeders 7 leechers 3 added 1710000000", got[0])
	}
```

`TestSolidTorrentsSearchMapsResults` — add:
```go
	if got[0].Seeders != 5 || got[0].Leechers != 1 || got[0].Added == 0 {
		t.Fatalf("solidtorrents fields = %+v, want seeders 5 leechers 1 added>0", got[0])
	}
```

`TestNyaaSearchParsesRSS` — add:
```go
	if got[0].Seeders != 11 || got[0].Leechers != 4 || got[0].Added == 0 {
		t.Fatalf("nyaa fields = %+v, want seeders 11 leechers 4 added>0", got[0])
	}
```

`TestSubsPleasePicksBestResolution` — add:
```go
	if got[0].Added == 0 {
		t.Fatalf("subsplease Added = 0, want the release_date parsed")
	}
```

`TestFetchWordpressRSSMapsMagnetItems` — add:
```go
	if got[0].Added == 0 {
		t.Fatalf("wordpress Added = 0, want the pubDate parsed")
	}
```

- [ ] **Step 2: Run tests to verify they fail.**

Run: `go test ./internal/source/ -run 'PirateBay|YTS|EZTV|SolidTorrents|Nyaa|SubsPlease|Wordpress' -v`
Expected: FAIL (fields are `0` / unknown identifier until the struct grows).

- [ ] **Step 3: Add the fields to `Result`.**

In `internal/source/source.go`, inside `type Result struct`, after `Popularity int64`:
```go
	Seeders    int64 // 0 when the source doesn't report it
	Leechers   int64 // 0 when the source doesn't report it
	Files      int   // 0 when unknown
	Added      int64 // unix seconds, 0 when unknown
```

- [ ] **Step 4: Populate each scraper (delete the `_ =` residue).**

`piratebay.go` — replace the trailing `_ = item.Leechers / NumFiles / Added` lines and set fields in the `Result{...}`:
```go
		leechers, _ := strconv.ParseInt(item.Leechers, 10, 64)
		files, _ := strconv.Atoi(item.NumFiles)
		added, _ := strconv.ParseInt(item.Added, 10, 64)
		out = append(out, Result{
			Title:      name,
			Source:     p.Label,
			SizeBytes:  size,
			Popularity: seeders,
			Seeders:    seeders,
			Leechers:   leechers,
			Files:      files,
			Added:      added,
			Category:   p.Category,
			Magnet:     buildMagnet(infoHash, name),
		})
```
Delete the three `_ = item.*` lines.

`yts.go` — set fields, delete `_ = movie.Uploaded` / `_ = tor.Peers`:
```go
			out = append(out, Result{
				Title:      name,
				Source:     "YTS",
				SizeBytes:  tor.SizeBytes,
				Popularity: tor.Seeds,
				Seeders:    tor.Seeds,
				Leechers:   tor.Peers,
				Added:      movie.Uploaded,
				Category:   "movies",
				Magnet:     buildMagnet(infoHash, name),
			})
```

`eztv.go` — set fields, delete `_ = tor.Peers` / `_ = tor.ReleasedAt`:
```go
		out = append(out, Result{
			Title:      name,
			Source:     "EZTV",
			SizeBytes:  size,
			Popularity: tor.Seeds,
			Seeders:    tor.Seeds,
			Leechers:   tor.Peers,
			Added:      tor.ReleasedAt,
			Category:   "tv",
			Magnet:     magnet,
		})
```

`solidtorrents.go` — set fields, replace `_ = item.Leechers` / `_ = parseTimeUnix(item.UpdatedAt)`:
```go
		out = append(out, Result{
			Title:      name,
			Source:     "Solid",
			SizeBytes:  item.Size,
			Popularity: item.Seeders,
			Seeders:    item.Seeders,
			Leechers:   item.Leechers,
			Added:      parseTimeUnix(item.UpdatedAt),
			Category:   "tv",
			Magnet:     buildMagnet(infoHash, name),
		})
```

`nyaa.go` — set fields, delete `_ = leechers` / `_ = parseTimeUnix(...)`:
```go
		out = append(out, Result{
			Title:      name,
			Source:     "Nyaa",
			SizeBytes:  parseSize(tag(item, "nyaa:size")),
			Popularity: seeders,
			Seeders:    seeders,
			Leechers:   leechers,
			Added:      parseTimeUnix(tag(item, "pubDate")),
			Category:   "anime",
			Magnet:     buildMagnet(infoHash, name),
		})
```

`x1337.go` — set `Seeders`/`Leechers`, delete `_ = row.Leechers`:
```go
		out = append(out, Result{
			Title:      row.Name,
			Source:     x.Label,
			SizeBytes:  row.SizeBytes,
			Popularity: row.Seeders,
			Seeders:    row.Seeders,
			Leechers:   row.Leechers,
			Category:   x.Category,
			Magnet:     parsed.Magnet,
		})
```

`subsplease.go` — set `Added`, delete `_ = parseTimeUnix(entry.ReleaseDate)`:
```go
		out = append(out, Result{
			Title:     name,
			Source:    "SubsPlease",
			SizeBytes: magnetXL(parsed.Magnet),
			Added:     parseTimeUnix(entry.ReleaseDate),
			Category:  "anime",
			Magnet:    parsed.Magnet,
		})
```

`torlink_helpers.go` — in `parseWordpressRSS`, set `Added` from the pubDate and drop the unused `sourceID` param. Change the signature and body:
```go
func parseWordpressRSS(xml, sourceLabel, category string) []Result {
	items := strings.Split(xml, "<item>")
	out := make([]Result, 0, len(items))
	for _, item := range items[1:] {
		magnet := firstSubmatch(rssMagnetRE, item)
		parsed := parseMagnet(magnet)
		if parsed == nil {
			continue
		}
		title := firstSubmatch(rssTitleRE, item)
		if title == "" {
			title = parsed.Name
		}
		out = append(out, Result{
			Title:    title,
			Source:   sourceLabel,
			Category: category,
			Added:    parseTimeUnix(firstSubmatch(rssDateRE, item)),
			Magnet:   parsed.Magnet,
		})
	}
	return out
}
```
Update `fetchWordpressRSS` to drop `sourceID` from its signature and its call:
```go
func fetchWordpressRSS(ctx context.Context, base, sourceLabel, category, query string, client *http.Client) ([]Result, error) {
	// ... unchanged body until the return ...
	return parseWordpressRSS(string(body), sourceLabel, category), nil
}
```
Update the caller in `fitgirl.go`:
```go
	return fetchWordpressRSS(ctx, f.Base, "FitGirl", "games", query, f.Client)
```
Update the test call in `torlink_test.go` `TestFetchWordpressRSSMapsMagnetItems`:
```go
	got, err := fetchWordpressRSS(context.Background(), srv.URL, "FitGirl", "games", "bunny", nil)
```

- [ ] **Step 5: Run source tests to verify they pass.**

Run: `go test ./internal/source/ -v`
Expected: PASS (all tests, including the tightened assertions).

- [ ] **Step 6: Checkpoint.**

Run: `go build ./... && go vet ./internal/source/`
Expected: no output, exit 0. (Commit if using git — no attribution trailer.)

---

### Task 2: `MultiSource.SearchStream`

**Files:**
- Modify: `internal/source/multi.go`
- Test: `internal/source/multi_test.go`

**Interfaces:**
- Consumes: `Result` (Task 1).
- Produces:
  - `type SourceUpdate struct { Results []Result; Err error; Done int; Total int }`
  - `func (m *MultiSource) SearchStream(ctx context.Context, query string, ch chan<- SourceUpdate)` — sends one update per source (ordered by completion), then closes `ch`. Consumed by Task 5.

- [ ] **Step 1: Write the failing test.**

Add to `internal/source/multi_test.go` (create if absent — it already exists). Use a tiny stub source defined in the test:
```go
func TestSearchStreamEmitsPerSourceAndCloses(t *testing.T) {
	ok1 := stubSource{name: "A", results: []Result{{Title: "a1", Popularity: 5}}}
	ok2 := stubSource{name: "B", results: []Result{{Title: "b1", Popularity: 9}}}
	bad := stubSource{name: "C", err: errors.New("boom")}
	m := NewMulti(ok1, ok2, bad)

	ch := make(chan SourceUpdate)
	go m.SearchStream(context.Background(), "q", ch)

	var updates []SourceUpdate
	for up := range ch {
		updates = append(updates, up)
	}
	if len(updates) != 3 {
		t.Fatalf("updates = %d, want 3", len(updates))
	}
	titles := map[string]bool{}
	sawErr := false
	maxDone := 0
	for _, up := range updates {
		if up.Total != 3 {
			t.Fatalf("Total = %d, want 3", up.Total)
		}
		if up.Done > maxDone {
			maxDone = up.Done
		}
		if up.Err != nil {
			sawErr = true
		}
		for _, r := range up.Results {
			titles[r.Title] = true
		}
	}
	if maxDone != 3 {
		t.Fatalf("max Done = %d, want 3", maxDone)
	}
	if !sawErr {
		t.Fatalf("expected the failing source to report Err")
	}
	if !titles["a1"] || !titles["b1"] {
		t.Fatalf("merged titles = %v, want a1 and b1", titles)
	}
}

type stubSource struct {
	name    string
	results []Result
	err     error
}

func (s stubSource) Name() string { return s.name }
func (s stubSource) Search(ctx context.Context, query string) ([]Result, error) {
	return s.results, s.err
}
```
Add `"errors"` to the test file imports.

- [ ] **Step 2: Run test to verify it fails.**

Run: `go test ./internal/source/ -run TestSearchStreamEmitsPerSource -v`
Expected: FAIL (`SearchStream` / `SourceUpdate` undefined).

- [ ] **Step 3: Implement `SearchStream`.**

In `internal/source/multi.go`, add `"sync/atomic"` to imports, then:
```go
// SourceUpdate is one source's contribution to a streaming search.
type SourceUpdate struct {
	Results []Result
	Err     error // this source's error, if any (non-fatal)
	Done    int   // sources finished so far, including this one
	Total   int   // total sources in the search
}

// SearchStream fans out like Search but sends each source's result on ch as it
// arrives (ordered by completion, not source order), then closes ch. A source
// error travels in SourceUpdate.Err and does not abort the others.
func (m *MultiSource) SearchStream(ctx context.Context, query string, ch chan<- SourceUpdate) {
	defer close(ch)
	total := len(m.sources)
	if total == 0 {
		return
	}
	var done int64
	var wg sync.WaitGroup
	for _, s := range m.sources {
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			res, err := s.Search(ctx, query)
			up := SourceUpdate{
				Results: res,
				Err:     err,
				Done:    int(atomic.AddInt64(&done, 1)),
				Total:   total,
			}
			select {
			case ch <- up:
			case <-ctx.Done(): // reader gave up (search superseded)
			}
		}(s)
	}
	wg.Wait()
}
```

- [ ] **Step 4: Run test to verify it passes.**

Run: `go test ./internal/source/ -run TestSearchStreamEmitsPerSource -v -race`
Expected: PASS (and no data race).

- [ ] **Step 5: Checkpoint.**

Run: `go test ./internal/source/ -race`
Expected: `ok`. (Commit if using git.)

---

### Task 3: UI sort + format helpers

**Files:**
- Modify: `internal/ui/helpers.go`
- Test: `internal/ui/helpers_test.go`

**Interfaces:**
- Consumes: `source.Result` (Task 1).
- Produces (consumed by Tasks 5, 7, 8, 9):
  - `type sortField int` with `sortNone, sortSize, sortSeeders, sortLeechers, sortRatio`
  - `var sortableCols = []sortField{sortSize, sortSeeders, sortLeechers, sortRatio}`
  - `func (f sortField) label() string`
  - `func applySort(rs []source.Result, f sortField, desc bool)` (in place, stable)
  - `func leechSeedRatio(r source.Result) float64`
  - `func relTime(unix int64) string`
  - `func seedLeech(r source.Result) string`
  - `func ratioStr(r source.Result) string`

- [ ] **Step 1: Write the failing tests.**

Add to `internal/ui/helpers_test.go`:
```go
func TestLeechSeedRatio(t *testing.T) {
	if got := leechSeedRatio(source.Result{Seeders: 10, Leechers: 5}); got != 0.5 {
		t.Fatalf("ratio = %v, want 0.5", got)
	}
	if got := leechSeedRatio(source.Result{Seeders: 0, Leechers: 3}); !math.IsInf(got, 1) {
		t.Fatalf("ratio with 0 seeders = %v, want +Inf", got)
	}
	if got := leechSeedRatio(source.Result{Seeders: 0, Leechers: 0}); got != 0 {
		t.Fatalf("ratio with no swarm = %v, want 0", got)
	}
}

func TestApplySort(t *testing.T) {
	rs := []source.Result{
		{Title: "a", SizeBytes: 100, Seeders: 1, Leechers: 9, Popularity: 1},
		{Title: "b", SizeBytes: 300, Seeders: 9, Leechers: 1, Popularity: 9},
		{Title: "c", SizeBytes: 200, Seeders: 5, Leechers: 0, Popularity: 5},
	}
	applySort(rs, sortSize, true) // desc
	if rs[0].Title != "b" || rs[2].Title != "a" {
		t.Fatalf("size desc order = %v", titles(rs))
	}
	applySort(rs, sortSeeders, false) // asc
	if rs[0].Title != "a" || rs[2].Title != "b" {
		t.Fatalf("seeders asc order = %v", titles(rs))
	}
	applySort(rs, sortNone, true) // by Popularity desc
	if rs[0].Title != "b" || rs[2].Title != "a" {
		t.Fatalf("default (popularity) order = %v", titles(rs))
	}
}

// NOTE: `titles([]source.Result) []string` already exists in model_test.go
// (same package `ui`) — reuse it; do NOT redeclare it here.

func TestRelTime(t *testing.T) {
	now := time.Now().Unix()
	cases := map[int64]string{
		0:                "",
		now - 30:         "just now",
		now - 3*3600:     "3h ago",
		now - 2*86400:    "2d ago",
		now - 400*86400:  "1y ago",
	}
	for in, want := range cases {
		if got := relTime(in); got != want {
			t.Errorf("relTime(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestSeedLeechAndRatioStr(t *testing.T) {
	if got := seedLeech(source.Result{Seeders: 69, Leechers: 12}); got != "69:12" {
		t.Fatalf("seedLeech = %q, want 69:12", got)
	}
	if got := seedLeech(source.Result{}); got != "—" {
		t.Fatalf("seedLeech (no data) = %q, want —", got)
	}
	if got := ratioStr(source.Result{Seeders: 10, Leechers: 5}); got != "0.50" {
		t.Fatalf("ratioStr = %q, want 0.50", got)
	}
	if got := ratioStr(source.Result{}); got != "—" {
		t.Fatalf("ratioStr (no data) = %q, want —", got)
	}
}
```
Ensure the test file imports `"math"`, `"time"`, and `"shoal/internal/source"`.

- [ ] **Step 2: Run tests to verify they fail.**

Run: `go test ./internal/ui/ -run 'LeechSeedRatio|ApplySort|RelTime|SeedLeech' -v`
Expected: FAIL (undefined identifiers).

- [ ] **Step 3: Implement the helpers.**

Add to `internal/ui/helpers.go` (add imports `"fmt"`, `"math"`, `"sort"`, `"time"`, `"shoal/internal/source"` as needed):
```go
type sortField int

const (
	sortNone sortField = iota
	sortSize
	sortSeeders
	sortLeechers
	sortRatio
)

var sortableCols = []sortField{sortSize, sortSeeders, sortLeechers, sortRatio}

func (f sortField) label() string {
	switch f {
	case sortSize:
		return "Size"
	case sortSeeders:
		return "Seeders"
	case sortLeechers:
		return "Leechers"
	case sortRatio:
		return "Ratio"
	default:
		return "Relevance"
	}
}

// leechSeedRatio is Leechers ÷ Seeders; 0 seeders with any leechers is +Inf
// (worst swarm), an empty swarm is 0.
func leechSeedRatio(r source.Result) float64 {
	if r.Seeders == 0 {
		if r.Leechers == 0 {
			return 0
		}
		return math.Inf(1)
	}
	return float64(r.Leechers) / float64(r.Seeders)
}

// applySort orders rs in place (stable). desc = largest/most first.
func applySort(rs []source.Result, f sortField, desc bool) {
	less := func(i, j int) bool {
		var a, b float64
		switch f {
		case sortSize:
			a, b = float64(rs[i].SizeBytes), float64(rs[j].SizeBytes)
		case sortSeeders:
			a, b = float64(rs[i].Seeders), float64(rs[j].Seeders)
		case sortLeechers:
			a, b = float64(rs[i].Leechers), float64(rs[j].Leechers)
		case sortRatio:
			a, b = leechSeedRatio(rs[i]), leechSeedRatio(rs[j])
		default: // sortNone → health order by Popularity
			a, b = float64(rs[i].Popularity), float64(rs[j].Popularity)
		}
		if desc {
			return a > b
		}
		return a < b
	}
	sort.SliceStable(rs, less)
}

func relTime(unix int64) string {
	if unix <= 0 {
		return ""
	}
	d := time.Since(time.Unix(unix, 0))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/24/365))
	}
}

func seedLeech(r source.Result) string {
	if r.Seeders == 0 && r.Leechers == 0 {
		return "—"
	}
	return fmt.Sprintf("%d:%d", r.Seeders, r.Leechers)
}

func ratioStr(r source.Result) string {
	if r.Seeders == 0 && r.Leechers == 0 {
		return "—"
	}
	v := leechSeedRatio(r)
	if math.IsInf(v, 1) {
		return "∞"
	}
	return fmt.Sprintf("%.2f", v)
}
```

- [ ] **Step 4: Run tests to verify they pass.**

Run: `go test ./internal/ui/ -run 'LeechSeedRatio|ApplySort|RelTime|SeedLeech' -v`
Expected: PASS. (If `relTime` rounding edges differ, the `400*86400 → "1y ago"` case pins the year bucket.)

- [ ] **Step 5: Checkpoint.**

Run: `go test ./internal/ui/`
Expected: `ok`. (Commit if using git.)

---

### Task 4: `titledBox` + width-aware padding helper

**Files:**
- Modify: `internal/ui/helpers.go`
- Test: `internal/ui/helpers_test.go`

**Interfaces:**
- Produces (consumed by Tasks 8, 9):
  - `func padVisual(s string, w int) string` — pads `s` with trailing spaces to visible width `w` (ANSI-aware via `lipgloss.Width`); returns `s` unchanged if already ≥ `w`.
  - `func titledBox(title, right, body string, width int, focused bool) string` — a rounded border of total width `width`, `title` inset in the top edge, optional right-aligned `right` label; accent border when `focused`, faint otherwise.

- [ ] **Step 1: Write the failing test.**

Add to `internal/ui/helpers_test.go` (import `"github.com/charmbracelet/lipgloss"` and `"strings"`):
```go
func TestTitledBoxDimensionsAndTitle(t *testing.T) {
	body := "hello\nworld"
	out := titledBox("Results (3)", "TPB", body, 30, true)
	lines := strings.Split(out, "\n")
	if len(lines) != 4 { // top border + 2 body + bottom border
		t.Fatalf("lines = %d, want 4:\n%s", len(lines), out)
	}
	for i, ln := range lines {
		if w := lipgloss.Width(ln); w != 30 {
			t.Fatalf("line %d width = %d, want 30: %q", i, w, ln)
		}
	}
	if !strings.Contains(lines[0], "Results (3)") {
		t.Fatalf("top border missing title: %q", lines[0])
	}
	if !strings.Contains(lines[0], "TPB") {
		t.Fatalf("top border missing right label: %q", lines[0])
	}
}

func TestPadVisual(t *testing.T) {
	if got := padVisual("hi", 5); lipgloss.Width(got) != 5 {
		t.Fatalf("padVisual width = %d, want 5", lipgloss.Width(got))
	}
	if got := padVisual("toolong", 3); got != "toolong" {
		t.Fatalf("padVisual over-width should pass through, got %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail.**

Run: `go test ./internal/ui/ -run 'TitledBox|PadVisual' -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement the helpers.**

Add to `internal/ui/helpers.go` (ensure `"strings"` and `"github.com/charmbracelet/lipgloss"` are imported):
```go
// padVisual right-pads s with spaces to visible width w (ANSI-aware). If s is
// already >= w it is returned unchanged.
// ponytail: callers keep body lines within the inner width; this only pads.
func padVisual(s string, w int) string {
	gap := w - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

// titledBox draws a rounded border of total width `width`, with `title` inset in
// the top edge and an optional right-aligned `right` label. Body lines are
// padded to the inner width.
func titledBox(title, right, body string, width int, focused bool) string {
	if width < 8 {
		width = 8
	}
	border := st.Faint
	if focused {
		border = st.Accent
	}
	inner := width - 2

	titleSeg := "─ " + title + " "
	var rightSeg string
	if right != "" {
		rightSeg = " " + right + " ─"
	}
	// top = ╭ + titleSeg + fill + rightSeg + ╮ , all == width
	fill := inner - lipgloss.Width(titleSeg) - lipgloss.Width(rightSeg)
	if fill < 0 {
		fill = 0
	}
	top := "╭" + titleSeg + strings.Repeat("─", fill) + rightSeg + "╮"

	var b strings.Builder
	b.WriteString(border.Render(top) + "\n")
	for _, ln := range strings.Split(body, "\n") {
		b.WriteString(border.Render("│") + padVisual(ln, inner) + border.Render("│") + "\n")
	}
	b.WriteString(border.Render("╰" + strings.Repeat("─", inner) + "╯"))
	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass.**

Run: `go test ./internal/ui/ -run 'TitledBox|PadVisual' -v`
Expected: PASS.

- [ ] **Step 5: Checkpoint.**

Run: `go test ./internal/ui/`
Expected: `ok`. (Commit if using git.)

---

### Task 5: Model — streaming search wiring

**Files:**
- Modify: `internal/ui/model.go`
- Test: `internal/ui/model_test.go`

**Interfaces:**
- Consumes: `source.SearchStream`/`SourceUpdate` (Task 2), `applySort` (Task 3).
- Produces (consumed by Tasks 7, 8):
  - `Model` fields: `searchCh chan source.SourceUpdate`, `sourcesDone int`, `sourcesTotal int`, `searchGen int`, `searchCancel context.CancelFunc`, plus sort state `sortMode bool`, `sortCol int`, `sortField sortField`, `sortDesc bool`.
  - `type sourceUpdateMsg struct { gen int; up source.SourceUpdate }`
  - `type searchClosedMsg struct { gen int }`
  - `func (m *Model) startSearch(query string) tea.Cmd`

- [ ] **Step 1: Write the failing test.**

Add to `internal/ui/model_test.go`:
```go
func TestStreamingUpdatesMergeAndCount(t *testing.T) {
	m := New(source.NewCurated(), nil)
	m.searchGen = 1
	m.searching = true
	m.searchCh = make(chan source.SourceUpdate, 1)

	// current-generation update: merges, sorts, records counts
	upd := sourceUpdateMsg{gen: 1, up: source.SourceUpdate{
		Results: []source.Result{{Title: "x", Popularity: 3}, {Title: "y", Popularity: 8}},
		Done:    1, Total: 2,
	}}
	m2, _ := m.Update(upd)
	mm := m2.(Model)
	if len(mm.results) != 2 || mm.sourcesDone != 1 || mm.sourcesTotal != 2 {
		t.Fatalf("after update: results=%d done=%d total=%d", len(mm.results), mm.sourcesDone, mm.sourcesTotal)
	}
	if mm.results[0].Title != "y" { // default sort = Popularity desc
		t.Fatalf("results not health-sorted: %s first", mm.results[0].Title)
	}

	// stale-generation update is ignored
	stale := sourceUpdateMsg{gen: 0, up: source.SourceUpdate{Results: []source.Result{{Title: "z"}}, Done: 9, Total: 9}}
	m3, _ := mm.Update(stale)
	if mmm := m3.(Model); len(mmm.results) != 2 || mmm.sourcesDone != 1 {
		t.Fatalf("stale update should be ignored, got results=%d done=%d", len(mmm.results), mmm.sourcesDone)
	}

	// close ends the spinner
	m4, _ := mm.Update(searchClosedMsg{gen: 1})
	if m4.(Model).searching {
		t.Fatalf("searchClosedMsg should stop searching")
	}
}
```

- [ ] **Step 2: Run test to verify it fails.**

Run: `go test ./internal/ui/ -run TestStreamingUpdates -v`
Expected: FAIL (undefined types/fields).

- [ ] **Step 3: Add state, messages, command, and Update cases.**

In `internal/ui/model.go`, add to the `Model` struct (near the search fields):
```go
	searchCh     chan source.SourceUpdate
	sourcesDone  int
	sourcesTotal int
	searchGen    int
	searchCancel context.CancelFunc

	sortMode  bool
	sortCol   int
	sortField sortField
	sortDesc  bool
```
In `NewWithConfig`, set the default sort direction on the returned `Model{...}` literal:
```go
		sortDesc: true,
```
Add the message types near `searchDoneMsg`:
```go
type sourceUpdateMsg struct {
	gen int
	up  source.SourceUpdate
}

type searchClosedMsg struct{ gen int }
```
Add the streaming interface and command near `searchCmd`:
```go
type streamSearcher interface {
	SearchStream(ctx context.Context, query string, ch chan<- source.SourceUpdate)
}

// startSearch begins a new (generation-tagged) search, streaming per-source
// updates when the source supports it, else falling back to a blocking search.
func (m *Model) startSearch(query string) tea.Cmd {
	if m.searchCancel != nil {
		m.searchCancel()
		m.searchCancel = nil
	}
	m.searchGen++
	m.results = nil
	m.cursor = 0
	m.sourcesDone = 0
	m.sourcesTotal = 0
	m.searching = true
	m.hasSearched = true

	ss, ok := m.src.(streamSearcher)
	if !ok {
		return searchCmd(m.src, query)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	m.searchCancel = cancel
	ch := make(chan source.SourceUpdate)
	m.searchCh = ch
	gen := m.searchGen
	go ss.SearchStream(ctx, query, ch)
	return waitForUpdate(gen, ch)
}

func waitForUpdate(gen int, ch chan source.SourceUpdate) tea.Cmd {
	return func() tea.Msg {
		up, ok := <-ch
		if !ok {
			return searchClosedMsg{gen: gen}
		}
		return sourceUpdateMsg{gen: gen, up: up}
	}
}
```
Add cases to `Update` (alongside `searchDoneMsg`):
```go
	case sourceUpdateMsg:
		if msg.gen != m.searchGen {
			return m, nil
		}
		if len(msg.up.Results) > 0 {
			m.results = append(m.results, msg.up.Results...)
			applySort(m.results, m.sortField, m.sortDesc)
		}
		m.sourcesDone = msg.up.Done
		m.sourcesTotal = msg.up.Total
		if n := len(m.filteredResults()); m.cursor >= n {
			m.cursor = max(0, n-1)
		}
		return m, waitForUpdate(msg.gen, m.searchCh)

	case searchClosedMsg:
		if msg.gen != m.searchGen {
			return m, nil
		}
		m.searching = false
		if len(m.results) == 0 {
			m.setNotice("No results.")
		}
		return m, nil
```

- [ ] **Step 4: Route the search box through `startSearch`.**

In `handleSearchEdit`, replace the `enter` success tail (the `m.searching = true` … `return m, searchCmd(m.src, q)` lines) with:
```go
		m.section = sectionSearch
		cmd := m.startSearch(q)
		return m, cmd
```

- [ ] **Step 5: Run tests to verify they pass.**

Run: `go test ./internal/ui/ -run TestStreamingUpdates -v`
Expected: PASS.

- [ ] **Step 6: Checkpoint.**

Run: `go build ./... && go test ./internal/ui/ -race`
Expected: build clean, `ok`. (Commit if using git.)

---

### Task 6: Model — detail screen + copy magnet

**Files:**
- Modify: `internal/ui/model.go`
- Test: `internal/ui/model_test.go`

**Interfaces:**
- Produces (consumed by Tasks 8, 9): `Model` fields `showDetail bool`, `detail source.Result`; package var `copyToClipboard func(string) error`.

- [ ] **Step 1: Write the failing test.**

Add to `internal/ui/model_test.go` (`errors`/`source` already imported there):
```go
func TestDetailOpenCopyAndBack(t *testing.T) {
	orig := copyToClipboard
	defer func() { copyToClipboard = orig }()

	const magnet = "magnet:?xt=urn:btih:abcdef0123456789abcdef0123456789abcdef01&dn=Movie"
	m := ready(New(&fakeSource{}, &fakeEngine{}))
	m.editing = false // command mode (ready model starts focused on the search box)
	m.hasSearched = true
	m.results = []source.Result{{Title: "Movie", Magnet: magnet}}
	m.cursor = 0

	// enter opens the detail screen
	m, _ = update(m, key("enter"))
	if !m.showDetail || m.detail.Title != "Movie" {
		t.Fatalf("enter did not open detail: showDetail=%v title=%q", m.showDetail, m.detail.Title)
	}

	// y copies the magnet via the injected clipboard func
	var copied string
	copyToClipboard = func(s string) error { copied = s; return nil }
	m, _ = update(m, key("y"))
	if copied != magnet {
		t.Fatalf("y did not copy magnet, copied=%q", copied)
	}
	if m.notice == "" {
		t.Fatalf("y should set a 'copied' notice")
	}

	// esc returns to the list
	m, _ = update(m, key("esc"))
	if m.showDetail {
		t.Fatalf("esc did not close detail")
	}
}
```

- [ ] **Step 2: Run test to verify it fails.**

Run: `go test ./internal/ui/ -run TestDetailOpenCopyAndBack -v`
Expected: FAIL (undefined fields / no detail handling).

- [ ] **Step 3: Add fields, the copy var, detail key handling, and open-on-enter.**

Add to the `Model` struct:
```go
	showDetail bool
	detail     source.Result
```
Add the package var (import `"github.com/atotto/clipboard"`):
```go
// copyToClipboard is a package var so tests can stub the system clipboard.
var copyToClipboard = clipboard.WriteAll
```
In `handleKey`, immediately after the `m.editingSetting` guard and before the "Command mode" switch, add:
```go
	if m.showDetail {
		switch msg.String() {
		case "esc":
			m.showDetail = false
		case "d":
			return m, addCmd(m.eng, m.detail)
		case "y":
			if err := copyToClipboard(m.detail.Magnet); err != nil {
				m.setError("Copy failed: " + err.Error())
			} else {
				m.setNotice("Magnet copied.")
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}
```
Change `activate` (the `sectionSearch` branch) to open the detail screen instead of downloading:
```go
	case sectionSearch:
		fr := m.filteredResults()
		if len(fr) > 0 && m.cursor < len(fr) {
			m.showDetail = true
			m.detail = fr[m.cursor]
		}
		return nil
```

- [ ] **Step 4: Run test to verify it passes.**

Run: `go test ./internal/ui/ -run TestDetailOpenCopyAndBack -v`
Expected: PASS.

- [ ] **Step 5: Promote the clipboard dependency.**

Run: `go mod tidy`
Expected: `github.com/atotto/clipboard` moves from `// indirect` to a direct require in `go.mod`.

- [ ] **Step 6: Checkpoint.**

Run: `go build ./... && go test ./internal/ui/`
Expected: build clean, `ok`. (Commit if using git.)

---

### Task 7: Model — sort mode keybindings

**Files:**
- Modify: `internal/ui/model.go`
- Test: `internal/ui/model_test.go`

**Interfaces:**
- Consumes: `sortableCols`, `applySort` (Task 3); sort state fields (Task 5).
- Produces: `func (m Model) handleSortKey(msg tea.KeyMsg) (tea.Model, tea.Cmd)`.

- [ ] **Step 1: Write the failing test.**

Add to `internal/ui/model_test.go`:
```go
func TestSortModeKeys(t *testing.T) {
	m := ready(New(&fakeSource{}, &fakeEngine{}))
	m.editing = false // command mode
	m.results = []source.Result{
		{Title: "a", SizeBytes: 1, Seeders: 1},
		{Title: "b", SizeBytes: 3, Seeders: 9},
		{Title: "c", SizeBytes: 2, Seeders: 5},
	}
	m.sortDesc = true

	// S enters sort mode and sorts by the first column (Size), desc → b first
	m, _ = update(m, key("S"))
	if !m.sortMode || m.sortField != sortSize || m.results[0].Title != "b" {
		t.Fatalf("S: sortMode=%v field=%v first=%s", m.sortMode, m.sortField, m.results[0].Title)
	}

	// right moves to Seeders (still desc → a last)
	m, _ = update(m, key("right"))
	if m.sortField != sortSeeders || m.results[2].Title != "a" {
		t.Fatalf("right: field=%v last=%s", m.sortField, m.results[2].Title)
	}

	// up sets ascending → a first
	m, _ = update(m, key("up"))
	if m.sortDesc || m.results[0].Title != "a" {
		t.Fatalf("up(asc): desc=%v first=%s", m.sortDesc, m.results[0].Title)
	}

	// esc exits sort mode
	m, _ = update(m, key("esc"))
	if m.sortMode {
		t.Fatalf("esc did not exit sort mode")
	}
}
```

- [ ] **Step 2: Run test to verify it fails.**

Run: `go test ./internal/ui/ -run TestSortModeKeys -v`
Expected: FAIL.

- [ ] **Step 3: Add sort-mode entry and handler.**

In `handleKey`, at the top of the "Command mode" section (right before `switch msg.String()`), add:
```go
	if m.sortMode {
		return m.handleSortKey(msg)
	}
```
Add a `"S"` case to the command-mode `switch`:
```go
	case "S":
		if m.section == sectionSearch {
			m.sortMode = true
			m.sortField = sortableCols[m.sortCol]
			applySort(m.results, m.sortField, m.sortDesc)
		}
		return m, nil
```
Add the handler method:
```go
func (m Model) handleSortKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "S", "esc", "enter":
		m.sortMode = false
	case "left", "h":
		if m.sortCol > 0 {
			m.sortCol--
		}
		m.sortField = sortableCols[m.sortCol]
		applySort(m.results, m.sortField, m.sortDesc)
	case "right", "l":
		if m.sortCol < len(sortableCols)-1 {
			m.sortCol++
		}
		m.sortField = sortableCols[m.sortCol]
		applySort(m.results, m.sortField, m.sortDesc)
	case "up", "k":
		m.sortDesc = false
		applySort(m.results, m.sortField, m.sortDesc)
	case "down", "j":
		m.sortDesc = true
		applySort(m.results, m.sortField, m.sortDesc)
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}
```

- [ ] **Step 4: Run test to verify it passes.**

Run: `go test ./internal/ui/ -run TestSortModeKeys -v`
Expected: PASS.

- [ ] **Step 5: Checkpoint.**

Run: `go test ./internal/ui/`
Expected: `ok`. (Commit if using git.)

---

### Task 8: View — results table

**Files:**
- Modify: `internal/ui/view.go`
- Test: `internal/ui/view_test.go`

**Interfaces:**
- Consumes: `titledBox`/`padVisual` (Task 4), `seedLeech`/`sortableCols`/`sortField.label` (Tasks 3), sort/stream state (Tasks 5, 7).
- Produces: a rewritten `renderResults(w, h int) string`.

- [ ] **Step 1: Write the failing test.**

Add to `internal/ui/view_test.go`:
```go
func TestRenderResultsTable(t *testing.T) {
	m := ready(New(&fakeSource{}, &fakeEngine{}))
	m.hasSearched = true
	m.results = []source.Result{
		{Title: "Magic Mike (2012) 1080p", Source: "TPB", SizeBytes: 1_700_000_000, Seeders: 69, Leechers: 12},
	}
	m.sourcesDone, m.sourcesTotal, m.searching = 6, 10, true
	m.sortField = sortSize
	m.sortDesc = true

	out := m.renderResults(80, 20)
	for _, want := range []string{"Results (1)", "searching… 6/10 sources", "Seed:Lch", "69:12", "TPB", "Size"} {
		if !strings.Contains(out, want) {
			t.Fatalf("renderResults missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "▼") { // active sort arrow (desc)
		t.Fatalf("renderResults missing sort arrow:\n%s", out)
	}
}

func TestRenderResultsSortBarInSortMode(t *testing.T) {
	m := ready(New(&fakeSource{}, &fakeEngine{}))
	m.hasSearched = true
	m.results = []source.Result{{Title: "x", Source: "TPB", Seeders: 1}}
	m.sortMode = true
	m.sortField = sortSeeders
	m.sortCol = 1

	out := m.renderResults(80, 20)
	if !strings.Contains(out, "Sort") || !strings.Contains(out, "Seeders") || !strings.Contains(out, "Leechers") {
		t.Fatalf("sort bar missing in sort mode:\n%s", out)
	}
}
```
Ensure `view_test.go` imports `"strings"` and `"shoal/internal/source"`.

- [ ] **Step 2: Run tests to verify they fail.**

Run: `go test ./internal/ui/ -run 'RenderResults' -v`
Expected: FAIL.

- [ ] **Step 3: Rewrite `renderResults`.**

Replace the whole `renderResults` function in `internal/ui/view.go` with:
```go
func (m Model) renderResults(w, h int) string {
	fr := m.filteredResults()
	if len(fr) == 0 && !m.searching {
		if m.filter != 0 {
			return "  " + st.Meta.Render("No matches in ") + st.Accent.Render(filterCats[m.filter].Label) +
				st.Meta.Render(". Try ") + st.Key.Render("← →") + st.Meta.Render(" for another filter.")
		}
		return "  " + st.Meta.Render("No matches. Try fewer or different words — or paste a magnet link.")
	}

	boxW := max(24, w)
	inner := boxW - 2

	// Column widths (right-aligned numeric columns; Name flexes). Leave a couple
	// columns of slack so no assembled row exceeds `inner` — titledBox pads short
	// lines but does not truncate long ones, so an over-long row would bow out the
	// right border.
	numW := max(2, len(strconv.Itoa(len(fr))))
	const sizeW, slW, srcW = 8, 9, 5
	nameW := max(6, inner-(numW+sizeW+slW+srcW+12))

	arrow := func(f sortField) string {
		if m.sortField != f {
			return ""
		}
		if m.sortDesc {
			return "▼"
		}
		return "▲"
	}
	colHead := func(label string, f sortField) string {
		a := arrow(f)
		if a != "" {
			return label + a
		}
		return label
	}

	var body strings.Builder

	if m.searching {
		body.WriteString(st.Meta.Render(fmt.Sprintf("searching… %d/%d sources", m.sourcesDone, m.sourcesTotal)) + "\n")
	}
	if m.sortMode {
		body.WriteString(m.renderSortBar() + "\n")
	}

	// Header row (prefix "  " matches the row's marker+space so Name aligns).
	head := "  " + strings.Repeat(" ", numW) + " " +
		st.Faint.Render(padRight("Name", nameW)) + " " +
		st.Faint.Render(leftPad(colHead("Size", sortSize), sizeW)) + "  " +
		st.Faint.Render(leftPad(colHead("Seed:Lch", sortSeeders), slW)) + "  " +
		st.Faint.Render(leftPad("Src", srcW))
	body.WriteString(head + "\n")

	const perItem = 1
	visible := max(1, (h-3)/perItem)
	start := 0
	if m.cursor >= visible {
		start = m.cursor - visible + 1
	}
	end := min(len(fr), start+visible)

	for i := start; i < end; i++ {
		r := fr[i]
		selected := i == m.cursor
		marker, nameStyle := " ", st.Row
		if selected {
			marker, nameStyle = st.Accent.Render(glyphCursor), st.RowSel
		}
		num := leftPad(strconv.Itoa(i+1), numW)
		name := padRight(truncate(r.Title, nameW), nameW)
		row := marker + " " + st.Faint.Render(num) + " " +
			nameStyle.Render(name) + " " +
			st.Meta.Render(leftPad(shortSize(r.SizeBytes), sizeW)) + "  " +
			st.Meta.Render(leftPad(seedLeech(r), slW)) + "  " +
			st.Meta.Render(leftPad(r.Source, srcW))
		body.WriteString(row)
		if i < end-1 {
			body.WriteString("\n")
		}
	}
	if end < len(fr) {
		// keep glyphMore — the existing TestRenderResultsListWithOverflow asserts it
		body.WriteString("\n" + st.Faint.Render(fmt.Sprintf("%s %d more %s", glyphMore, len(fr)-end, glyphDown)))
	}

	title := fmt.Sprintf("Results (%d)", len(fr))
	return titledBox(title, "", body.String(), boxW, m.section == sectionSearch)
}

func (m Model) renderSortBar() string {
	parts := make([]string, 0, len(sortableCols))
	for i, f := range sortableCols {
		lbl := f.label()
		if i == m.sortCol {
			dir := "▼"
			if !m.sortDesc {
				dir = "▲"
			}
			parts = append(parts, st.Accent.Render("[ "+lbl+" "+dir+" ]"))
		} else {
			parts = append(parts, st.Faint.Render(lbl))
		}
	}
	return st.SectionHead.Render("Sort ▸") + " " + strings.Join(parts, "   ")
}
```
Add these small helpers to `view.go` (near `sizeOrDash`):
```go
func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func leftPad(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}

// shortSize is a compact size for the table column (e.g. "1.7G", "751M").
func shortSize(n int64) string {
	if n <= 0 {
		return "—"
	}
	const u = 1024.0
	f := float64(n)
	switch {
	case f >= u*u*u*u:
		return fmt.Sprintf("%.1fT", f/(u*u*u*u))
	case f >= u*u*u:
		return fmt.Sprintf("%.1fG", f/(u*u*u))
	case f >= u*u:
		return fmt.Sprintf("%.0fM", f/(u*u))
	case f >= u:
		return fmt.Sprintf("%.0fK", f/u)
	default:
		return fmt.Sprintf("%dB", n)
	}
}
```
Add `"strconv"` to `view.go` imports.

- [ ] **Step 4: Run tests to verify they pass.**

Run: `go test ./internal/ui/ -run 'RenderResults' -v`
Expected: PASS.

- [ ] **Step 5: Checkpoint.**

Run: `go build ./... && go test ./internal/ui/`
Expected: build clean, `ok`. (Commit if using git.)

---

### Task 9: View — detail screen, wiring, footer hints

**Files:**
- Modify: `internal/ui/view.go`
- Test: `internal/ui/view_test.go`

**Interfaces:**
- Consumes: `titledBox` (Task 4), `relTime`/`seedLeech` (Task 3), `parseMagnet` (via `source`), detail state (Task 6).
- Produces: `renderDetail(w, h int) string`; `renderMain`/`renderSearch` route to it; footer gains sort/detail hints.

- [ ] **Step 1: Write the failing test.**

Add to `internal/ui/view_test.go`:
```go
func TestRenderDetail(t *testing.T) {
	m := ready(New(&fakeSource{}, &fakeEngine{}))
	m.showDetail = true
	m.detail = source.Result{
		Title:     "Magic Mike (2012) 1080p BrRip x264 - YIFY",
		Source:    "TPB",
		SizeBytes: 1_700_000_000,
		Seeders:   69,
		Leechers:  12,
		Files:     3,
		Added:     time.Now().Add(-48 * time.Hour).Unix(),
		Magnet:    "magnet:?xt=urn:btih:1681fba79fa80d6db6916975e8dafb637058c87b&dn=Magic",
	}

	out := m.renderDetail(80, 24)
	// Note: the seeder count and " seeders …" are separately styled spans, so
	// assert on the contiguous "seeders · N leechers" run, not "69 seeders".
	for _, want := range []string{"Details", "Magic Mike", "Size", "Health", "seeders · 12 leechers", "Files", "Added", "Hash", "1681fba7", "Magnet", "d Download", "y Copy magnet", "esc back"} {
		if !strings.Contains(out, want) {
			t.Fatalf("renderDetail missing %q:\n%s", want, out)
		}
	}
}
```
Ensure `view_test.go` imports `"time"`.

- [ ] **Step 2: Run test to verify it fails.**

Run: `go test ./internal/ui/ -run TestRenderDetail -v`
Expected: FAIL.

- [ ] **Step 3: Implement `renderDetail` and route to it.**

Add to `internal/ui/view.go` (add `"shoal/internal/source"` to imports):
```go
func (m Model) renderDetail(w, h int) string {
	r := m.detail
	boxW := max(24, w)

	var b strings.Builder
	b.WriteString(st.Row.Render(truncate(r.Title, boxW-4)) + "\n\n")

	row := func(label, val string) {
		if val == "" {
			return
		}
		b.WriteString(st.Faint.Render(padRight(label, 8)) + " " + val + "\n")
	}

	row("Size", st.Row.Render(sizeOrDash(r.SizeBytes)))
	row("Health", detailHealth(r))
	if r.Files > 0 {
		row("Files", st.Row.Render(fmt.Sprintf("%d", r.Files)))
	}
	row("Added", st.Meta.Render(relTime(r.Added)))
	if pm := source.ParseMagnetInfoHash(r.Magnet); pm != "" {
		row("Hash", st.Faint.Render(pm))
	}
	if r.Magnet != "" {
		row("Magnet", st.Faint.Render(truncate(r.Magnet, boxW-14)))
	} else if r.TorrentURL != "" {
		row("Torrent", st.Faint.Render(truncate(r.TorrentURL, boxW-14)))
	}

	b.WriteString("\n")
	b.WriteString(st.Key.Render("d") + " " + st.KeyDesc.Render("Download") + "   " + st.FooterSep.Render("·") + "   ")
	b.WriteString(st.Key.Render("y") + " " + st.KeyDesc.Render("Copy magnet") + "   " + st.FooterSep.Render("·") + "   ")
	b.WriteString(st.Key.Render("esc") + " " + st.KeyDesc.Render("back"))

	query := st.SearchLabel.Render("❯ ") + truncate(m.input.Value(), boxW-6)
	search := titledBox("Search", "", query, boxW, false)
	details := titledBox("Details", r.Source, b.String(), boxW, true)
	return search + "\n" + details
}

func detailHealth(r source.Result) string {
	if r.Seeders == 0 && r.Leechers == 0 {
		if r.Popularity > 0 {
			return st.Meta.Render(fmt.Sprintf("%s downloads", thousands(r.Popularity)))
		}
		return st.Meta.Render("—")
	}
	return st.Good.Render(fmt.Sprintf("%d", r.Seeders)) + st.Meta.Render(fmt.Sprintf(" seeders · %d leechers", r.Leechers))
}
```
In `internal/source`, add a tiny exported helper (magnet parsing already exists internally). Add to `torlink_helpers.go`:
```go
// ParseMagnetInfoHash returns the 40-char hex infohash from a magnet URI, or ""
// when the input isn't a magnet the client understands.
func ParseMagnetInfoHash(magnet string) string {
	if pm := parseMagnet(magnet); pm != nil {
		return pm.InfoHash
	}
	return ""
}
```
Route to the detail view: in `renderMain`, change the `default`/`sectionSearch` path so detail wins:
```go
	default:
		if m.showDetail {
			return m.renderDetail(w, h)
		}
		return m.renderSearch(w, h)
```

- [ ] **Step 4: Update the footer hints.**

In `renderFooter`, replace the `m.section == sectionSearch` case and add sort/detail cases:
```go
	case m.showDetail:
		parts = []string{hint("d", "download"), hint("y", "copy magnet"), hint("esc", "back")}
	case m.sortMode:
		parts = []string{hint("←→", "column"), hint("↑↓", "direction"), hint("esc", "done")}
	case m.section == sectionSearch:
		parts = []string{
			hint("/", "search"), hint("↑↓", "move"), hint("←→", "filter"),
			hint("enter", "details"), hint("d", "download"), hint("S", "sort"),
			hint("tab", "panes"), hint("?", "help"), hint("q", "quit"),
		}
```
(Place the `m.showDetail` and `m.sortMode` cases before the `m.section == sectionSearch` case in the `switch`.)

- [ ] **Step 4b: Update the help screen rows to match the new keys.**

In `helpView` (`internal/ui/view.go`), replace the `rows` slice with:
```go
	rows := [][2]string{
		{"/", "focus the search box and start typing"},
		{"enter", "run the search · open a result's details"},
		{"esc", "leave the search box / close details / cancel"},
		{"↑ ↓ / k j", "move the selection"},
		{"← → / h l", "switch the media filter · change a setting"},
		{"d", "download the selected result"},
		{"S", "sort results (←→ column · ↑↓ direction)"},
		{"y", "copy magnet (in details)"},
		{"tab", "cycle Search · Downloads · Seeding · Settings"},
		{"?", "toggle this help"},
		{"q / ctrl+c", "quit"},
	}
```

- [ ] **Step 5: Run tests to verify they pass.**

Run: `go test ./internal/ui/ -run TestRenderDetail -v`
Expected: PASS.

- [ ] **Step 6: Full checkpoint.**

Run: `go build ./... && go vet ./... && go test ./... -race && gofmt -l internal/`
Expected: build clean, vet clean, all tests `ok`, `gofmt -l` prints nothing. (Commit if using git.)

---

## Self-Review Notes

- **Spec coverage:** Result fields + population (Task 1); streaming (Task 2); sort/format helpers + ratio div-by-zero (Task 3); titledBox (Task 4); model streaming with stale-gen guard (Task 5); detail + copy via injectable `copyToClipboard` (Task 6); modal sort keys (Task 7); results table with subtitle/sort-bar/arrows (Task 8); detail render + footer + `go.mod` promote (Task 9). All spec sections mapped.
- **Type consistency:** `sortField`/`sortableCols`/`applySort`/`leechSeedRatio` (Task 3) reused verbatim in Tasks 5/7/8; `sourceUpdateMsg`/`searchClosedMsg`/`startSearch`/`waitForUpdate` defined once (Task 5) and consumed in tests; `titledBox`/`padVisual` (Task 4) reused in Tasks 8/9; `source.SourceUpdate`/`SearchStream` (Task 2) matched by the `streamSearcher` interface (Task 5); `ParseMagnetInfoHash` added in Task 9.
- **Known follow-ups (not in scope):** the existing `addedMsg`/`addCmd`/`searchDoneMsg` fallback path stays for single sources; `renderResults` right-alignment uses byte-length padding on plain (unstyled) cells, which is correct for ASCII columns (Name truncation uses the existing rune-safe `truncate`).

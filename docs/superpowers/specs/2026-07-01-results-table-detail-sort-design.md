# Search results: tabular view, detail screen, live source progress, sortable columns

**Date:** 2026-07-01
**Component:** `internal/ui`, `internal/source`
**Status:** Approved (design)

## Summary

Replace shoal's two-line search-results list with a bordered, columnar table;
add a torrent **detail screen**; show **live per-source progress** while a
multi-source search streams in; and make the results table **sortable** by
Size, Seeders, Leechers, and Ratio.

The existing chrome stays: left sidebar (Search / Downloads / Seeding /
Settings) and the top header/footer are unchanged. Only the Search pane's body
and its keybindings change.

## Goals

- Results render as a titled box `Results (N)` with columns `# · Name · Size · Seed:Lch · Src`.
- Selecting a result (`enter`) opens a **Details** screen (Size, Health, Files, Added, Hash, Magnet) with actions `d` download, `y` copy magnet, `esc` back.
- While searching, the box subtitle shows `searching… D/M sources`, and results appear incrementally as each source returns.
- A modal **sort** picker (`S`) lets the user sort by Size / Seeders / Leechers / Ratio, ascending or descending, applied live.

## Non-goals (YAGNI)

- No full bordered-pane redesign of the other panes (sidebar stays).
- No pagination beyond the existing scroll window.
- No mouse/column-click sorting, no resizable panes, no saved sort preference across sessions.

## 1. Data model — `source.Result`

The scrapers already parse seeders/leechers/file-counts/dates and then discard
them (the `_ = tor.Peers`, `_ = item.NumFiles`, `_ = parseTimeUnix(...)`
residue). This work wires that data through instead of throwing it away.

Add to `Result`:

```go
Seeders  int64 // 0 when the source doesn't report it
Leechers int64 // 0 when the source doesn't report it
Files    int   // 0 when unknown
Added    int64 // unix seconds, 0 when unknown
```

`InfoHash` is **derived on demand** from `Magnet` via the existing
`parseMagnet` (used only by the detail screen's Hash row) — no stored field.

Per-source population (replaces the discarded values; removes the `_ =` lines):

| Source | Seeders | Leechers | Files | Added |
|---|---|---|---|---|
| PirateBay | `seeders` | `leechers` | `num_files` | `added` (unix) |
| 1337x | `Seeders` | `Leechers` | — | — |
| YTS | `seeds` | `peers` | — | `date_uploaded_unix` |
| EZTV | `seeds` | `peers` | — | `date_released_unix` |
| SolidTorrents | `seeders` | `leechers` | — | `updatedAt` (via `parseTimeUnix`) |
| Nyaa | `seeders` | `leechers` | — | `pubDate` |
| SubsPlease | — | — | — | `release_date` |
| FitGirl / WordPress RSS | — | — | — | `pubDate` (currently discarded in `parseWordpressRSS`) |
| Internet Archive | — | — | — | — (keeps `Popularity` = downloads) |
| Curated | — | — | — | — |

Sorting/health ordering still uses `Popularity` as the default; the new fields
drive the columns, detail screen, and explicit sorts. Sources with no seed/leech
data render `—` in `Seed:Lch` (matches the `—` rows in the target mockup).

Also drop the now-dead `sourceID` parameter threaded through
`fetchWordpressRSS`/`parseWordpressRSS` (unused after this change).

## 2. Streaming search — `MultiSource`

Add a streaming method alongside the existing blocking `Search` (kept for
single sources and as a fallback):

```go
// SourceUpdate is one source's contribution to a streaming search.
type SourceUpdate struct {
    Results []Result
    Err     error // this source's error, if any (non-fatal)
    Done    int   // sources finished so far (including this one)
    Total   int   // total sources in the search
}

// SearchStream fans out like Search but sends each source's result on ch as it
// arrives (ordered by completion, not source order), then closes ch.
func (m *MultiSource) SearchStream(ctx context.Context, query string, ch chan<- SourceUpdate)
```

Implementation: one goroutine per source; each sends a `SourceUpdate` on
completion with `Done` = an atomically-incremented counter and `Total` =
`len(sources)`; a coordinating goroutine closes `ch` once all have sent. The
blocking `Search` remains for single-source use and for the model's fallback
path.

**Error handling:** a source's error travels in `SourceUpdate.Err` and is
non-fatal (its `Results` are empty). The model surfaces results from the sources
that succeeded; only if *every* source failed does it show a search-failed
notice (parity with today's `Search`).

## 3. Model wiring — `internal/ui/model.go`

New state on `Model`:

```go
// streaming search
searchCh     chan source.SourceUpdate
sourcesDone  int
sourcesTotal int
searchGen    int                 // bumped per search; stale msgs ignored
searchCancel context.CancelFunc  // cancels the in-flight streaming ctx

// detail screen
showDetail bool
detail     source.Result

// sort
sortMode  bool      // modal column picker active
sortCol   int       // highlighted sortable column index (in sort mode)
sortField sortField // active sort (sortNone = default health order)
sortDesc  bool
```

Where `sortField` is an enum: `sortNone, sortSize, sortSeeders, sortLeechers, sortRatio`.

New messages/commands:

- `sourceUpdateMsg{gen int; up source.SourceUpdate}` and `searchClosedMsg{gen int}`.
- `startStreamSearch(query)`: bump `searchGen`; cancel any prior `searchCancel`; make a fresh `ctx`/`cancel` and `searchCh`. If `m.src` implements the streaming interface, launch `SearchStream` in a goroutine and return a `waitForUpdate` command; otherwise fall back to the existing `searchCmd`.
- `waitForUpdate(gen, ch)`: reads one value; returns `sourceUpdateMsg` (channel open) or `searchClosedMsg` (channel closed).

Update handling:

- `sourceUpdateMsg`: if `gen != m.searchGen`, ignore (stale). Otherwise append `up.Results` to `m.results`, re-apply the active sort, set `sourcesDone/sourcesTotal`, and return `waitForUpdate` again to pump the next value. Keep `m.cursor` clamped to the current list length.
- `searchClosedMsg`: if `gen == m.searchGen`, set `searching=false`; if there are no results, show the "No results" / "Search failed" notice.

The streaming detection uses a local interface (`interface{ SearchStream(...) }`)
so single sources keep working via the blocking path.

### Interaction changes

- **Search list `enter`** → `showDetail = true`, `detail =` selected result (was: download). List `d` still quick-downloads.
- **Detail screen keys:** `d` → `addCmd`; `y` → copy magnet; `esc` → `showDetail=false`.
- **Copy magnet** goes through an injectable indirection `copyFn func(string) error` (default `clipboard.WriteAll`, already an available dependency) so tests can stub it. Success/failure sets a notice.
- **Sort mode:** `S` toggles `sortMode`. While active, arrows are captured for sorting instead of navigation:
  - `←`/`→` move `sortCol` across the sortable columns (Size, Seeders, Leechers, Ratio), setting `sortField` to the highlighted one.
  - `↑` sets ascending, `↓` sets descending.
  - `esc` / `enter` / `S` exit sort mode. The chosen sort persists and re-applies to streamed results.

`applySort(results)` sorts stably by the active field/direction. **Ratio** =
`Leechers / Seeders`; guard `Seeders == 0` by treating the ratio as `+Inf` so
those rows sort consistently. `sortNone` leaves the merged `Popularity`-desc
order untouched.

## 4. View — `internal/ui/view.go`

**`titledBox(title, rightLabel, body string, width int, focused bool) string`** —
one reusable helper: a rounded border with `title` inset in the top edge and an
optional right-aligned `rightLabel` (e.g. `TPB`), accent-coloured when
`focused`. Used by both the Results box and the Details box.

**Results** (`renderResults`): a `titledBox` titled `Results (N)`. When
searching, a subtitle line `searching… D/M sources`. When `sortMode`, a sort bar:

```
Sort ▸  Size   [ Seeders ▼ ]   Leechers   Ratio
```

Then a dim header row and data rows:

```
 # Name                                   Size  Seed:Lch  Src
) 1 Magic Mike (2012) 1080p BrRip x264 …  1.7G     69:12  TPB
  2 Magic Mike XXL (2015) 1080p BrRip …   1.8G     50:2   TPB
```

- Columns: `#` (width = digits of largest index, min 2), `Size` (right, ~9),
  `Seed:Lch` (right, ~9), `Src` (right, ~5); `Name` flexes to the remaining
  width and is truncated with `…`.
- The active sort column's header gets a `▲`/`▼` arrow (even outside sort mode).
- Selected row: `)` cursor + accent style. Keep the windowed scroll and the
  `… N more ↓` footer.
- `Seed:Lch` shows `seeders:leechers`, or `—` when there is no seed data.

**Detail** (`renderDetail`): the query line/box, then a `titledBox` titled
`Details` with the `Src` label in the top-right border, containing:

```
Magic Mike (2012) 1080p BrRip x264 - YIFY

Size     1.70 GB
Health   69 seeders · 12 leechers      (fallback: "N downloads" when no seed data; "—" if neither)
Files    3                             (omitted when 0/unknown)
Added    13y ago                       (omitted when unknown)
Hash     1681fba79fa80d6db6916975e8dafb637058c87b   (omitted when no magnet)
Magnet   magnet:?xt=urn:btih:…                       (truncated with …)

d Download   ·   y Copy magnet   ·   esc back
```

New formatting helpers: `relTime(unix int64) string` (`just now`, `5h ago`,
`2d ago`, `3mo ago`, `13y ago`), `seedLeech(r Result) string`,
`ratioStr(r Result) string`, and a right-align/pad helper for columns.

**Footer** (`renderFooter`): Search-pane hints gain `S sort`; sort mode shows
`←→ column · ↑↓ dir · esc done`; the detail screen shows its own hints.

## 5. Testing (TDD — tests written first)

**`internal/source`**
- Extend `torlink_test.go` assertions: PirateBay sets `Files`/`Added`; YTS/EZTV set `Seeders`/`Leechers`/`Added`; Nyaa/SolidTorrents set `Seeders`/`Leechers`/`Added`; SubsPlease/FitGirl set `Added`.
- New `multi_test.go` `SearchStream` test: 3 sources (2 succeed, 1 errors) → assert `Total==3`, updates arrive with monotonically increasing `Done` up to 3, the erroring source contributes `Err` but no results, merged results contain both good sources' hits, and `ch` closes.

**`internal/ui`**
- `helpers_test.go`: table tests for `relTime`, `seedLeech`, `ratioStr`, and `applySort` ordering — including the `Seeders == 0` ratio (→ `+Inf`) edge and ascending/descending.
- `view_test.go`: `renderResults` output contains the header, a `Seed:Lch` cell, and the sort arrow on the active column; the sort bar appears only in sort mode; `renderDetail` contains Size/Health/Hash lines and the action footer.
- `model_test.go`: `enter` in the results list sets `showDetail`; `esc` clears it; `y` invokes the injected `copyFn` and sets a notice; a `sourceUpdateMsg` with the current `gen` merges results and updates `sourcesDone/Total`, while one with a stale `gen` is ignored; `S` toggles sort mode and arrows change `sortField`/`sortDesc`.

## Error handling summary

- All sources fail → "Search failed" notice; partial failure → show what returned.
- Stale streaming updates (superseded search) dropped via `searchGen`.
- Copy-magnet error → error notice; detail with no magnet omits Hash/Magnet rows.
- Ratio sort guards divide-by-zero (`Seeders == 0` → `+Inf`).

## Files touched

- `internal/source/source.go` — new `Result` fields.
- `internal/source/multi.go` — `SourceUpdate`, `SearchStream`.
- `internal/source/{piratebay,x1337,yts,eztv,solidtorrents,nyaa,subsplease}.go` and `torlink_helpers.go` — populate fields, remove `_ =` residue + dead `sourceID` param.
- `internal/ui/model.go` — streaming/detail/sort state, messages, commands, keybindings.
- `internal/ui/view.go` — `titledBox`, table renderer, detail renderer, formatting helpers, footer.
- `go.mod` — promote `github.com/atotto/clipboard` from indirect to direct.
- Tests as above.

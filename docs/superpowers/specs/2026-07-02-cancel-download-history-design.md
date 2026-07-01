# Cancel downloads + persistent download history

**Date:** 2026-07-02
**Component:** `internal/engine`, `internal/history` (new), `internal/ui`, `cmd/shoal`
**Status:** Approved (design)

## Summary

Two features for shoal's Downloads/Seeding panes:

1. **Cancel a download** — select an in-progress download and cancel it, choosing at
   confirm time whether to keep or delete the partial data on disk.
2. **Download history** — a persistent record of completed downloads, shown as a
   section inside the Seeding pane and surviving restarts.

## Goals

- Select a download in the Downloads pane (`↑↓`) and press `x` to cancel it.
- The cancel confirm prompt offers the choice: `k` keep partial files, `d` delete them, `esc` abort.
- Completed downloads are recorded (name, size, when) and persist across restarts.
- The Seeding pane shows active seeding first, then a History section of completed downloads.

## Non-goals (YAGNI)

- No cancel for seeding torrents (Downloads pane only).
- No per-file selection within a torrent.
- No clear/edit/remove of history entries (easy to add later).
- No new History nav pane — history folds into Seeding per the approved design.

## 1. Engine — identify and remove torrents

`engine.Status` today carries only `Name`; the UI can't address a specific
torrent. Add its infohash, and a removal method.

```go
// Status gains:
InfoHash string // lowercase hex infohash; "" only while metadata is still fetching

// Engine interface gains:
// Remove stops the torrent with the given hex infohash and forgets it. When
// deleteData is true, its downloaded file/dir under the data dir is also
// removed from disk. Unknown hashes are a no-op (nil error).
Remove(infoHash string, deleteData bool) error
```

`Anacrolix` implementation:
- Store `dataDir string` on the struct (from `Config.DataDir`) — needed to locate files for deletion.
- `Statuses()` fills `InfoHash: h.HexString()`.
- `Remove(infoHash, deleteData)`: lock `a.mu`; scan `a.client.Torrents()` for the matching `t.InfoHash().HexString()`; capture `name` (from `a.names[h]` or `t.Name()`); `t.Drop()`; `delete(a.names, h)`, `delete(a.addedAt, h)`; if `deleteData && name != ""`, `os.RemoveAll(filepath.Join(a.dataDir, name))` (covers both the single-file and multi-file layouts, whose top-level entry is named after the torrent). Return `nil` when the hash isn't found.

`fakeEngine` (UI test double) implements `Remove` by recording `removedHash`,
`removedDelete`, and returning a settable `removeErr`.

## 2. Cancel flow (Downloads pane) — `internal/ui`

- **Selection cursor:** the Downloads pane has none today. Add a `dlCursor int`
  and extend `moveUp`/`moveDown` to move it over `downloading()` (clamped to
  `[0, len-1]`). The Downloads sidebar entry already shows the count.
- **Cancel + confirm state:** add `cancelConfirm bool` and `cancelTarget engine.Status`.
  - In the Downloads section (not confirming), `x` with a valid selection sets
    `cancelConfirm = true`, `cancelTarget = downloading()[dlCursor]`.
  - A modal guard `if m.cancelConfirm { return m.handleCancelKey(msg) }` sits in
    `handleKey` among the other modal guards (after `editing`/`editingSetting`/
    `showHelp`/`showDetail`/`sortMode`, before the command switch).
  - `handleCancelKey`: `k` → `removeCmd(hash, false)`; `d` → `removeCmd(hash, true)`;
    `esc`/`n` → abort. Each clears `cancelConfirm`.
- **Command:** `removeCmd(eng, infoHash string, deleteData bool) tea.Cmd` calls
  `eng.Remove` and returns `removedMsg{name string, err error}`. `Update` shows a
  notice: success (`"Cancelled: <name>"` / `"Deleted: <name>"`) or error. After
  removal, `dlCursor` re-clamps on the next render/tick.
- **Render (`renderDownloads`):** highlight the selected row (accent cursor,
  matching the results-table selection style). While `cancelConfirm`, render the
  prompt line: `Cancel "<name>"?  k keep files · d delete files · esc`.

## 3. History store — `internal/history` (new package)

```go
type Entry struct {
    InfoHash    string    `json:"info_hash"`
    Name        string    `json:"name"`
    Size        int64     `json:"size"`
    CompletedAt time.Time `json:"completed_at"`
}

type Store struct {
    Path    string  // history.json in the OS config dir; "" disables Save (tests)
    Entries []Entry // newest first
}

func Load() Store              // reads <config-dir>/shoal/history.json (empty on any error)
func LoadFrom(path string) Store
func (s *Store) Append(e Entry) // dedup by InfoHash (ignore if present), prepend newest-first, then Save
func (s Store) Save() error     // no-op when Path == ""; else MarshalIndent to Path (0o755 dir, 0o644 file)
```

- Path resolution mirrors `internal/config` (`os.UserConfigDir()` + `shoal/history.json`).
- `Append` dedups by `InfoHash` so re-recording the same torrent is a no-op.
- Corrupt/missing file → empty `Entries` (never fatal).

## 4. History detection + wiring — `internal/ui`, `cmd/shoal`

- `Model` gains `history history.Store`. It is injected, not loaded in `New`
  (keeps tests hermetic): add `func (m Model) WithHistory(h history.Store) Model`.
  `cmd/shoal/main.go` calls `ui.NewWithConfig(...).WithHistory(history.Load())`.
  Tests set `m.history` directly (same package) or leave it empty (`Path==""`,
  `Save` no-ops).
- **Detection on tick:** reuse the prev/next snapshot (same pattern as the speed
  sampler). Build `prevDone` from `m.statuses` keyed by `InfoHash`; for each
  `next` status that is `Done` and whose infohash was not `Done` before, call
  `m.history.Append(history.Entry{InfoHash, Name, Size: TotalBytes, CompletedAt: tickTime})`.
  `Append`'s dedup makes repeats harmless (including a torrent that is already
  Done on first sight). A helper `newlyCompleted(prev, next []engine.Status) []engine.Status`
  keeps this testable.
- Entries with an empty `InfoHash` (metadata still fetching — cannot happen for a
  completed torrent, but guard anyway) are skipped.

## 5. Seeding pane renders history — `internal/ui`

`renderSeeding` shows two sections:
- **Active seeding** — the existing `m.seeding()` rows (live).
- **History** — `m.history.Entries`, excluding any infohash currently in
  `m.seeding()` (dedup active vs past), rendered newest first as
  `✓ <name>  ·  <size>  ·  <relTime(CompletedAt.Unix())>` under a `HISTORY`
  subheader.
- Empty handling: if there is neither active seeding nor history, keep the
  existing "Nothing seeding yet…" hint. If only one section has content, show
  just that section.

## 6. Footer / help

- Downloads footer gains `↑↓ move` and `x cancel`.
- Cancel-confirm footer: `k keep · d delete · esc cancel`.
- `helpView` gains a row: `x  cancel the selected download (keep or delete files)`.

## Error handling

- `Remove` on an unknown/already-gone hash → `nil` (no-op); UI still shows the
  outcome notice for the name it had.
- File deletion errors surface via `removedMsg.err` → error notice; the torrent
  is still dropped from the client.
- History file read/write errors are non-fatal: a failed `Load` yields empty
  history; a failed `Save` is ignored (best-effort log store).
- `dlCursor` is clamped whenever `downloading()` shrinks (cancel, completion).

## Testing (TDD)

**`internal/engine`**
- `Remove` on a tracked offline torrent drops it from `Statuses()` (extends the
  existing offline-`.torrent` harness; `t.Skip` if a client can't start, matching
  the existing lifecycle test). `Remove` on an unknown hash returns `nil`.

**`internal/history`**
- Round-trip: `Append` two entries → `Save` → `LoadFrom` returns them newest-first;
  a duplicate `InfoHash` is ignored; a missing file loads empty. Uses a temp path.

**`internal/ui`**
- `fakeEngine.Remove` records hash + deleteData.
- Downloads `dlCursor` moves with `↑↓` and clamps.
- `x` opens the confirm; `k`/`d` return a command that calls `Remove` with
  `deleteData` false/true; `esc` aborts without calling `Remove`.
- `newlyCompleted` returns only torrents that flipped `Done` false→true.
- A tick that flips a torrent to `Done` appends exactly one history entry
  (and a second identical tick does not duplicate it).
- `renderDownloads` shows the confirm prompt while `cancelConfirm`.
- `renderSeeding` shows a completed history entry (and dedups one that is also
  actively seeding).

## Files touched

- `internal/engine/engine.go` — `Status.InfoHash`, `Engine.Remove`.
- `internal/engine/anacrolix.go` — `dataDir` field, `InfoHash` fill, `Remove` impl.
- `internal/engine/anacrolix_test.go` — `Remove` test.
- `internal/history/history.go` + `history_test.go` — new package.
- `internal/ui/model.go` — `dlCursor`, cancel-confirm state/keys, `removeCmd`/`removedMsg`, history field + `WithHistory`, tick detection.
- `internal/ui/view.go` — Downloads selection + confirm prompt, Seeding history section, footer/help.
- `internal/ui/model_test.go`, `internal/ui/view_test.go` — tests + `fakeEngine.Remove`.
- `cmd/shoal/main.go` — pass `Config.DataDir` to the engine (if not already) and wire `WithHistory(history.Load())`.

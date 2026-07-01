# shoal

A terminal BitTorrent client in Go, built in two layers:

- **Phase 1 — the protocol, by hand.** The entire download path (reading a `.torrent`,
  talking to a tracker, the peer handshake, trading and verifying pieces) is written
  from scratch using only the Go standard library. This is the part you build to
  *understand* how torrents actually work. It lives in the repo-root packages and
  `cmd/shoal-classic`.
- **Phase 2 — a real, beautiful tool.** A calm, fullscreen terminal UI
  ([Bubble Tea](https://github.com/charmbracelet/bubbletea)) that searches a
  torlink-style source set and downloads with a full BitTorrent engine
  ([anacrolix/torrent](https://github.com/anacrolix/torrent) — DHT, magnets, web
  seeds, seeding). This is `cmd/shoal` and `internal/`.

Same idea as torlink (the Node client this project learns from): a search layer, an
engine, and a TUI. Phase 1 is the engine written by hand; Phase 2 swaps in a mature
engine and adds the search + UI.

## Quick start

> **First time:** the Phase 2 TUI uses a few well-known libraries. Fetch them once:
>
> ```sh
> cd shoal
> make deps          # go get bubbletea/bubbles/lipgloss/anacrolix + go mod tidy
> ```
>
> This writes `go.mod` + `go.sum`. Commit both — CI needs them to pass.

```sh
make run            # build and launch the fullscreen TUI
# or:
make build && ./shoal

make test           # run the Phase 1 unit tests (offline, no deps needed)
make classic        # build the hand-written CLI downloader (./shoal-classic)
```

Run `make help` for all targets.

## Using the TUI

It opens fullscreen on the **Search** pane. Keys (also shown in the footer, and `?`
brings up the full list):

| Key | Action |
| --- | --- |
| `/` | focus the search box and type |
| `enter` | run the search · or download the selected result |
| `↑ ↓` / `k j` | move the selection |
| `d` | download the selected result |
| `tab` | switch between Search and Downloads |
| `?` | toggle help |
| `q` / `ctrl+c` | quit |

Search hits stream from the configured sources; pick one and press `d`, and it moves to
**Downloads** with a live progress bar, byte counts, and peer count. You can also paste
a magnet link into the search box and press enter to add it directly. Files land in
`~/Downloads/shoal`.

Default sources are Internet Archive, a small open-media catalogue, and the
torlink-derived providers: FitGirl, YTS, The Pirate Bay, 1337x, EZTV,
SolidTorrents, Nyaa, and SubsPlease.

## The hand-written core (Phase 1)

BitTorrent has no central server holding the file. Everyone sharing a file forms a
**swarm**, and you assemble the file from pieces handed to you by many peers at once:

1. **Read the `.torrent`** — a "bencoded" dictionary. Its nested `info` dictionary has
   the name, the fixed **piece length**, and the SHA-1 of every piece. The torrent's
   **infohash** is the SHA-1 of the raw `info` bytes. → `bencode/`, `metainfo/`.
2. **Ask a tracker for peers** — the phonebook: send the infohash, get back peer
   `IP:port` addresses. → `tracker/`.
3. **Handshake with a peer** — a fixed 68-byte greeting, then a tiny message protocol
   (`bitfield`, `interested`/`unchoke`, `request`, `piece`). → `peer/`.
4. **Download and verify pieces** — request 16 KiB blocks, reassemble, check each
   piece's SHA-1, spread the work across all peers, retry failures. → `download/`.
5. **Write to disk** — single file, or a directory tree for multi-file torrents.
   → `cmd/shoal-classic/`.

Try it standalone with `make classic && ./shoal-classic some.torrent .` (HTTP-tracker
torrents only — see [Phase 1 limitations](#phase-1-limitations)).

## How this maps to torlink

| Layer | torlink (Node) | shoal |
| --- | --- | --- |
| BitTorrent engine | `webtorrent` | Phase 1: **hand-written**; Phase 2: `anacrolix/torrent` |
| Search sources | `src/sources/*` | `internal/source` (Internet Archive + torlink-derived providers) |
| UI | Ink / React | `internal/ui` (Bubble Tea) |

## Project layout

```
shoal/
├── bencode/            # Phase 1: .torrent / tracker encoding
├── metainfo/           # Phase 1: parse a .torrent, compute the infohash
├── tracker/            # Phase 1: HTTP tracker announce
├── peer/               # Phase 1: handshake, wire messages, bitfield
├── download/           # Phase 1: concurrent verified piece downloader
├── internal/
│   ├── source/         # Phase 2: Source interface + default provider set
│   ├── engine/         # Phase 2: Engine interface + anacrolix backend
│   └── ui/             # Phase 2: the fullscreen Bubble Tea interface
└── cmd/
    ├── shoal/          # Phase 2: the TUI (the product)
    └── shoal-classic/  # Phase 1: the hand-written CLI downloader
```

The Phase 1 packages have tests that run offline (`make test`). The decoupling is
deliberate: the UI depends only on the `source.Source` and `engine.Engine`
interfaces, so the engine can be swapped without touching the UI.

## Phase 1 limitations

The hand-written engine is a teaching tool and stays simple on purpose: HTTP trackers
only (no UDP), no DHT/PEX, no magnet links, expects a peer's opening `bitfield`, no
completion timeout, and download-only (never seeds). Phase 2 (anacrolix) has none of
these limits — it's there precisely so the real tool is robust while the hand-written
code stays readable.

## Roadmap

**Phase 1 — protocol by hand.** ✅ bencode, metainfo + infohash, HTTP tracker, peer
handshake + messages, concurrent verified piece download, CLI.

**Phase 2 — a real tool.** ✅ anacrolix engine, `Source` interface + Internet Archive
search, fullscreen Bubble Tea TUI with live progress. Next:

- Persist the download queue across runs (mirroring torlink's `queue.ts`).
- A Seeding pane with pause/resume.
- More sources behind the same `Source` interface.
- Ship as a single static binary (the big payoff over torlink — no runtime to install).

## A note on use

BitTorrent itself is neutral infrastructure — Linux distributions, game patches, and
large open datasets are all distributed this way. What carries legal risk is the
*content* and the *indexing sites*, not the protocol. shoal can search general
torrent indexes by default; use it only for content you have the right to download
and share.

## License

MIT — do what you like.

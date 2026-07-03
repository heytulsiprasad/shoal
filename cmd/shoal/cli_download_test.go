package main

import (
	"path/filepath"
	"testing"

	"github.com/StrangeNoob/shoal/internal/engine"
	"github.com/StrangeNoob/shoal/internal/queue"
)

func TestCurrentStatusFindsByInfoHash(t *testing.T) {
	fake := fakeStatusEngine{statuses: []engine.Status{
		{InfoHash: "aaa", Name: "other"},
		{InfoHash: "bbb", Name: "target"},
	}}
	st, ok := currentStatus(fake, "bbb")
	if !ok || st.Name != "target" {
		t.Fatalf("currentStatus = %+v, %v; want target, true", st, ok)
	}
	if _, ok := currentStatus(fake, "missing"); ok {
		t.Fatalf("currentStatus found a hash that isn't present")
	}
}

// TestResolveInfoHashMatchesPersistedEntry covers the bug this replaced: the
// prior implementation diffed engine.Statuses() before/after Add*, which
// silently never matched when target was already in the queue (e.g. resuming
// a torrent left paused by a previous --timeout). resolveInfoHash instead
// reads the queue entry Add* just persisted, so it works whether or not the
// entry already existed.
func TestResolveInfoHashMatchesPersistedEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q := queue.LoadFrom(path)
	q.Upsert(queue.Entry{InfoHash: "already-queued", Magnet: "magnet:?xt=urn:btih:aaa", Name: "resumed"})
	q.Upsert(queue.Entry{InfoHash: "other", TorrentURL: "https://example.test/other.torrent"})

	// Simulates a fresh add whose target matches an entry already on disk
	// before this call — the scenario that broke the old before/after diff.
	hash, err := resolveInfoHash(downloadTarget{magnet: "magnet:?xt=urn:btih:aaa"}, path)
	if err != nil || hash != "already-queued" {
		t.Fatalf("resolveInfoHash(magnet) = %q, %v; want already-queued, nil", hash, err)
	}

	hash, err = resolveInfoHash(downloadTarget{torrentURL: "https://example.test/other.torrent"}, path)
	if err != nil || hash != "other" {
		t.Fatalf("resolveInfoHash(torrentURL) = %q, %v; want other, nil", hash, err)
	}

	if _, err := resolveInfoHash(downloadTarget{magnet: "magnet:?xt=urn:btih:nope"}, path); err == nil {
		t.Fatal("resolveInfoHash: want error for a target never persisted")
	}
}

// TestPrepareTargetResumesAlreadyQueuedEntry covers a real bug caught during
// manual verification: restore() re-adds a persisted paused entry and calls
// engine.Pause on it (SetMaxEstablishedConns(0) at the anacrolix level).
// AddMagnet/AddTorrentURL on an already-tracked info hash just return the
// existing handle — they never lift that cap. Without an explicit Resume
// call, a target resuming from a prior --timeout/Ctrl-C would sit at 0 peers
// forever even though resolveInfoHash correctly finds it. Live-verified: a
// 263.6MB download timed out at 72.73%, and re-running the same command
// without this fix stayed at 0 peers/0 bytes for a full 35s before the fix
// made it resume instantly and finish.
func TestPrepareTargetResumesAlreadyQueuedEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	q := queue.LoadFrom(path)
	q.Upsert(queue.Entry{InfoHash: "resumed-hash", Magnet: "magnet:?xt=urn:btih:aaa", Name: "resumed", Paused: true})

	fake := &resumeTrackingEngine{}
	target := downloadTarget{magnet: "magnet:?xt=urn:btih:aaa", name: "resumed"}

	hash, err := prepareTarget(fake, target, path)
	if err != nil || hash != "resumed-hash" {
		t.Fatalf("prepareTarget = %q, %v; want resumed-hash, nil", hash, err)
	}
	if !fake.resumed {
		t.Fatal("prepareTarget did not call eng.Resume — a target resumed from a prior timeout/Ctrl-C would stay stuck at 0 peer connections forever")
	}
	if fake.resumedHash != "resumed-hash" {
		t.Fatalf("Resume called with wrong hash %q; want resumed-hash", fake.resumedHash)
	}
}

type fakeStatusEngine struct {
	statuses []engine.Status
}

func (f fakeStatusEngine) AddTorrentURL(string, string) error { return nil }
func (f fakeStatusEngine) AddMagnet(string) error             { return nil }
func (f fakeStatusEngine) Statuses() []engine.Status          { return f.statuses }
func (f fakeStatusEngine) Remove(string, bool) error          { return nil }
func (f fakeStatusEngine) Pause(string) error                 { return nil }
func (f fakeStatusEngine) Resume(string) error                { return nil }
func (f fakeStatusEngine) Close() error                       { return nil }

type resumeTrackingEngine struct {
	fakeStatusEngine
	resumed     bool
	resumedHash string
}

func (f *resumeTrackingEngine) Resume(hash string) error {
	f.resumed = true
	f.resumedHash = hash
	return nil
}

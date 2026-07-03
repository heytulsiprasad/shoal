package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/StrangeNoob/shoal/internal/history"
	"github.com/StrangeNoob/shoal/internal/queue"
)

func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(configDir, "shoal")
}

func TestRunStatusJSON(t *testing.T) {
	configDir := isolateConfig(t)
	q := queue.LoadFrom(queue.DefaultPath())
	q.Upsert(queue.Entry{InfoHash: "aaa", Magnet: "magnet:?xt=urn:btih:aaa", Name: "Queued", Paused: true})
	h := history.LoadFrom(filepath.Join(configDir, "history.json"))
	h.Append(history.Entry{InfoHash: "bbb", Name: "Done", Size: 123, CompletedAt: time.Unix(2000, 0), Path: "/tmp/done"})

	var out bytes.Buffer
	if code := runStatus([]string{"--json"}, &out); code != 0 {
		t.Fatalf("runStatus code = %d, want 0; output %q", code, out.String())
	}

	var got struct {
		Queued    []queue.Entry   `json:"queued"`
		Completed []history.Entry `json:"completed"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("status json: %v\n%s", err, out.String())
	}
	if len(got.Queued) != 1 || got.Queued[0].InfoHash != "aaa" || !got.Queued[0].Paused {
		t.Fatalf("queued = %+v", got.Queued)
	}
	if len(got.Completed) != 1 || got.Completed[0].InfoHash != "bbb" || got.Completed[0].Path != "/tmp/done" {
		t.Fatalf("completed = %+v", got.Completed)
	}
}

func TestRunStatusHuman(t *testing.T) {
	isolateConfig(t)
	q := queue.LoadFrom(queue.DefaultPath())
	q.Upsert(queue.Entry{InfoHash: "aaa", TorrentURL: "https://example.test/a.torrent", Name: "Queued"})

	var out bytes.Buffer
	if code := runStatus(nil, &out); code != 0 {
		t.Fatalf("runStatus code = %d, want 0; output %q", code, out.String())
	}
	if !strings.Contains(out.String(), "Queued") || !strings.Contains(out.String(), "Recent completed") {
		t.Fatalf("human status output = %q", out.String())
	}
}

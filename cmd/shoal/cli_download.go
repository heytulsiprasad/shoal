package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/StrangeNoob/shoal/internal/config"
	"github.com/StrangeNoob/shoal/internal/engine"
	"github.com/StrangeNoob/shoal/internal/history"
	shlock "github.com/StrangeNoob/shoal/internal/lock"
	"github.com/StrangeNoob/shoal/internal/queue"
)

type downloadOptions struct {
	category string
	index    int
	timeout  time.Duration
	open     bool
	json     bool
}

type downloadTarget struct {
	torrentURL string
	magnet     string
	name       string
	seeders    int64
}

type downloadSummary struct {
	Status         string  `json:"status"`
	InfoHash       string  `json:"info_hash"`
	Name           string  `json:"name"`
	Path           string  `json:"path"`
	SizeBytes      int64   `json:"size_bytes"`
	Seeders        int64   `json:"seeders"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	Error          string  `json:"error,omitempty"`
}

func runDownload(args []string, out io.Writer) int {
	opts, targetArg, err := parseDownloadArgs(args)
	if err != nil {
		return downloadError(out, wantsJSON(args), err, 1)
	}

	target, err := resolveDownloadTarget(targetArg, opts)
	if err != nil {
		return downloadError(out, opts.json, err, 1)
	}

	l, err := acquireAppLock()
	if err != nil {
		if pid, ok := heldPID(err); ok {
			msg := heldLockMessage(pid)
			fmt.Fprintln(os.Stderr, msg)
			if opts.json {
				_ = writeDownloadSummary(out, downloadSummary{Status: "error", Error: msg})
			}
			return 2
		}
		return downloadError(out, opts.json, err, 1)
	}
	defer l.Release()

	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return downloadError(out, opts.json, err, 1)
	}

	eng, err := engine.NewAnacrolix(engine.Config{
		DataDir:    cfg.DataDir,
		ListenPort: cfg.ListenPort,
		MaxPeers:   cfg.MaxPeers,
		Seed:       cfg.Seed,
		SeedRatio:  cfg.SeedRatio,
		QueuePath:  queue.DefaultPath(),
	})
	if err != nil {
		return downloadError(out, opts.json, fmt.Errorf("starting torrent engine: %w", err), 1)
	}
	defer eng.Close()

	infoHash, err := prepareTarget(eng, target, queue.DefaultPath())
	if err != nil {
		return downloadError(out, opts.json, err, 1)
	}

	return monitorDownload(eng, infoHash, target, opts, out)
}

// prepareTarget adds target to eng and returns the info hash the engine
// assigned it, resuming it if it was left paused by a prior --timeout or
// Ctrl-C.
//
// Add* upserts the queue entry to disk synchronously before returning
// (queue.Store.Upsert calls Save() inline), so the info hash is already
// resolvable via resolveInfoHash right after — deterministically, unlike
// diffing Statuses() before/after, which breaks when target was already
// queued.
//
// A target left paused comes back from restore() re-added but still paused
// (SetMaxEstablishedConns(0)) — Add* only returns the existing handle
// without lifting that, so it would sit at 0 peers forever unless
// explicitly resumed here.
func prepareTarget(eng engine.Engine, target downloadTarget, queuePath string) (string, error) {
	switch {
	case target.torrentURL != "":
		if err := eng.AddTorrentURL(target.torrentURL, target.name); err != nil {
			return "", err
		}
	case target.magnet != "":
		if err := eng.AddMagnet(target.magnet); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("nothing to download")
	}

	infoHash, err := resolveInfoHash(target, queuePath)
	if err != nil {
		return "", fmt.Errorf("resolving added torrent: %w", err)
	}
	if err := eng.Resume(infoHash); err != nil {
		return "", fmt.Errorf("resuming torrent: %w", err)
	}
	return infoHash, nil
}

// resolveInfoHash finds the info hash the engine assigned to target by
// matching the queue entry it just persisted. A few retries guard against
// filesystem scheduling jitter, though Upsert's Save() happens synchronously
// inside Add* so the first read should always hit.
func resolveInfoHash(target downloadTarget, path string) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		q := queue.LoadFrom(path)
		for _, e := range q.Entries {
			if target.torrentURL != "" && e.TorrentURL == target.torrentURL {
				return e.InfoHash, nil
			}
			if target.magnet != "" && e.Magnet == target.magnet {
				return e.InfoHash, nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", fmt.Errorf("added torrent not found in queue")
}

func parseDownloadArgs(args []string) (downloadOptions, string, error) {
	var opts downloadOptions
	fs := flag.NewFlagSet("shoal download", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.category, "category", "", "filter category")
	fs.IntVar(&opts.index, "index", 0, "result index")
	fs.DurationVar(&opts.timeout, "timeout", 0, "download timeout")
	fs.BoolVar(&opts.open, "open", false, "open when done")
	fs.BoolVar(&opts.json, "json", false, "print JSON")

	pos, err := parseInterspersed(fs, args, map[string]bool{"category": true, "index": true, "timeout": true})
	if err != nil {
		return opts, "", err
	}
	if opts.index < 0 {
		return opts, "", fmt.Errorf("--index must be >= 0")
	}
	target := strings.TrimSpace(strings.Join(pos, " "))
	if target == "" {
		return opts, "", fmt.Errorf("missing query, magnet, or torrent URL")
	}
	return opts, target, nil
}

func resolveDownloadTarget(arg string, opts downloadOptions) (downloadTarget, error) {
	switch {
	case strings.HasPrefix(strings.ToLower(arg), "magnet:"):
		return downloadTarget{magnet: arg, name: "magnet link"}, nil
	case isHTTPURL(arg):
		return downloadTarget{torrentURL: arg, name: torrentNameFromURL(arg)}, nil
	}

	results, err := searchRanked(arg, opts.category)
	if err != nil {
		return downloadTarget{}, fmt.Errorf("search failed: %w", err)
	}
	if len(results) == 0 {
		return downloadTarget{}, fmt.Errorf("no results found for %q", arg)
	}
	if opts.index >= len(results) {
		return downloadTarget{}, fmt.Errorf("--index %d out of range (%d results)", opts.index, len(results))
	}
	r := results[opts.index]
	switch {
	case r.TorrentURL != "":
		return downloadTarget{torrentURL: r.TorrentURL, name: r.Title, seeders: r.Seeders}, nil
	case r.Magnet != "":
		return downloadTarget{magnet: r.Magnet, name: r.Title, seeders: r.Seeders}, nil
	default:
		return downloadTarget{}, fmt.Errorf("%q has no torrent URL or magnet", r.Title)
	}
}

func isHTTPURL(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func torrentNameFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err == nil {
		if base := path.Base(u.Path); base != "." && base != "/" && base != "" {
			return strings.TrimSuffix(base, ".torrent")
		}
	}
	return "torrent"
}

func monitorDownload(eng engine.Engine, infoHash string, target downloadTarget, opts downloadOptions, out io.Writer) int {
	start := time.Now()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var timeout <-chan time.Time
	var timer *time.Timer
	if opts.timeout > 0 {
		timer = time.NewTimer(opts.timeout)
		timeout = timer.C
		defer timer.Stop()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	var last engine.Status
	for {
		if st, ok := currentStatus(eng, infoHash); ok {
			last = st
			if !opts.json {
				printProgress(out, st)
			}
			if st.Done {
				now := time.Now()
				h := history.Load()
				h.Append(history.Entry{InfoHash: st.InfoHash, Name: st.Name, Size: st.TotalBytes, CompletedAt: now, Path: st.Path})
				if opts.open && st.Path != "" {
					if err := openPath(st.Path); err != nil {
						fmt.Fprintln(os.Stderr, "shoal: open:", err)
					}
				}
				if opts.json {
					_ = writeDownloadSummary(out, summaryFromStatus("done", st, infoHash, target, start, ""))
				} else {
					fmt.Fprintln(out, "done:", st.Path)
				}
				return 0
			}
		}

		select {
		case <-ticker.C:
		case <-timeout:
			_ = eng.Pause(infoHash)
			if opts.json {
				_ = writeDownloadSummary(out, summaryFromStatus("timeout", last, infoHash, target, start, ""))
			} else {
				fmt.Fprintln(out, "timeout reached; torrent is still queued")
			}
			return 3
		case <-sigCh:
			if opts.json {
				_ = writeDownloadSummary(out, summaryFromStatus("interrupted", last, infoHash, target, start, ""))
			} else {
				fmt.Fprintln(out, "interrupted, still queued")
			}
			return 130
		}
	}
}

// currentStatus looks up infoHash directly — resolveInfoHash already pinned
// down the exact hash before monitoring started, so no before/after guessing
// is needed here.
func currentStatus(eng engine.Engine, infoHash string) (engine.Status, bool) {
	for _, s := range eng.Statuses() {
		if s.InfoHash == infoHash {
			return s, true
		}
	}
	return engine.Status{}, false
}

func printProgress(out io.Writer, s engine.Status) {
	fmt.Fprintf(out, "%6.2f%%  %s / %s  %d peers\n",
		s.Percent()*100,
		humanBytes(s.CompletedBytes),
		humanBytes(s.TotalBytes),
		s.Peers,
	)
}

func summaryFromStatus(status string, s engine.Status, infoHash string, target downloadTarget, start time.Time, errText string) downloadSummary {
	hash := s.InfoHash
	if hash == "" {
		hash = infoHash
	}
	name := s.Name
	if name == "" {
		name = target.name
	}
	return downloadSummary{
		Status:         status,
		InfoHash:       hash,
		Name:           name,
		Path:           s.Path,
		SizeBytes:      s.TotalBytes,
		Seeders:        target.seeders,
		ElapsedSeconds: time.Since(start).Seconds(),
		Error:          errText,
	}
}

func writeDownloadSummary(out io.Writer, s downloadSummary) error {
	return json.NewEncoder(out).Encode(s)
}

func downloadError(out io.Writer, jsonMode bool, err error, code int) int {
	if jsonMode {
		_ = writeDownloadSummary(out, downloadSummary{Status: "error", Error: err.Error()})
	} else {
		fmt.Fprintln(os.Stderr, "shoal: download:", err)
	}
	return code
}

func wantsJSON(args []string) bool {
	for _, arg := range args {
		if arg == "--json" || arg == "-json" || strings.HasPrefix(arg, "--json=") || strings.HasPrefix(arg, "-json=") {
			return true
		}
	}
	return false
}

func openPath(p string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", p)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", p)
	default:
		cmd = exec.Command("xdg-open", p)
	}
	return cmd.Start()
}

func acquireAppLock() (*shlock.Lock, error) {
	return shlock.Acquire(configLockPath())
}

func configLockPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "shoal", "shoal.lock")
}

func heldPID(err error) (int, bool) {
	var held shlock.HeldError
	if errors.As(err, &held) {
		return held.PID, true
	}
	return 0, false
}

func heldLockMessage(pid int) string {
	return fmt.Sprintf("shoal is already running (pid %d) — close the TUI or other shoal command first", pid)
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/StrangeNoob/shoal/internal/history"
	"github.com/StrangeNoob/shoal/internal/queue"
)

type statusOptions struct {
	json bool
}

func runStatus(args []string, out io.Writer) int {
	opts, err := parseStatusArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shoal: status:", err)
		return 1
	}

	q := queue.LoadFrom(queue.DefaultPath())
	h := history.Load()
	if opts.json {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			Queued    []queue.Entry   `json:"queued"`
			Completed []history.Entry `json:"completed"`
		}{Queued: q.Entries, Completed: h.Entries}); err != nil {
			fmt.Fprintln(os.Stderr, "shoal: status:", err)
			return 1
		}
		return 0
	}

	fmt.Fprintln(out, "Queued")
	if len(q.Entries) == 0 {
		fmt.Fprintln(out, "  none")
	} else {
		for i, e := range q.Entries {
			state := "queued"
			if e.Paused {
				state = "paused"
			}
			fmt.Fprintf(out, "  %d. %-7s %-12s %s\n", i+1, state, shortHash(e.InfoHash), e.Name)
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Recent completed")
	if len(h.Entries) == 0 {
		fmt.Fprintln(out, "  none")
	} else {
		for i, e := range h.Entries {
			fmt.Fprintf(out, "  %d. %-12s %-9s %-16s %s\n",
				i+1,
				shortHash(e.InfoHash),
				humanBytes(e.Size),
				formatStatusTime(e.CompletedAt),
				e.Name,
			)
		}
	}
	return 0
}

func parseStatusArgs(args []string) (statusOptions, error) {
	var opts statusOptions
	fs := flag.NewFlagSet("shoal status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&opts.json, "json", false, "print JSON")
	pos, err := parseInterspersed(fs, args, nil)
	if err != nil {
		return opts, err
	}
	if len(pos) > 0 {
		return opts, fmt.Errorf("unexpected argument %q", pos[0])
	}
	return opts, nil
}

func shortHash(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}

func formatStatusTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/StrangeNoob/shoal/internal/source"
)

type searchOptions struct {
	category string
	limit    int
	json     bool
}

type searchJSONResult struct {
	Rank       int    `json:"rank"`
	Title      string `json:"title"`
	Source     string `json:"source"`
	SizeBytes  int64  `json:"size_bytes"`
	Popularity int64  `json:"popularity"`
	Seeders    int64  `json:"seeders"`
	Leechers   int64  `json:"leechers"`
	Files      int    `json:"files"`
	Added      int64  `json:"added"`
	Category   string `json:"category"`
	TorrentURL string `json:"torrent_url"`
	Magnet     string `json:"magnet"`
}

func runSearch(args []string, out io.Writer) int {
	opts, query, err := parseSearchArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shoal: search:", err)
		return 1
	}

	results, err := searchRanked(query, opts.category)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shoal: search failed:", err)
		return 1
	}
	results = limitResults(results, opts.limit)

	if opts.json {
		if err := writeSearchJSON(out, results); err != nil {
			fmt.Fprintln(os.Stderr, "shoal: search:", err)
			return 1
		}
		return 0
	}
	if len(results) == 0 {
		fmt.Fprintln(out, "no results found")
		return 0
	}
	writeSearchTable(out, results, terminalWidth())
	return 0
}

func parseSearchArgs(args []string) (searchOptions, string, error) {
	var opts searchOptions
	fs := flag.NewFlagSet("shoal search", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.category, "category", "", "filter category")
	fs.IntVar(&opts.limit, "limit", 0, "limit results")
	fs.BoolVar(&opts.json, "json", false, "print JSON")

	pos, err := parseInterspersed(fs, args, map[string]bool{"category": true, "limit": true})
	if err != nil {
		return opts, "", err
	}
	if opts.limit < 0 {
		return opts, "", fmt.Errorf("--limit must be >= 0")
	}
	query := strings.TrimSpace(strings.Join(pos, " "))
	if query == "" {
		return opts, "", fmt.Errorf("missing query")
	}
	return opts, query, nil
}

func parseInterspersed(fs *flag.FlagSet, args []string, valueFlags map[string]bool) ([]string, error) {
	var flags []string
	var pos []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flags = append(flags, arg)
			if valueFlags[flagName(arg)] && !strings.Contains(arg, "=") && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, arg)
	}
	if err := fs.Parse(flags); err != nil {
		return nil, err
	}
	return pos, nil
}

func flagName(arg string) string {
	name := strings.TrimLeft(arg, "-")
	if i := strings.IndexByte(name, '='); i >= 0 {
		name = name[:i]
	}
	return name
}

func searchRanked(query, category string) ([]source.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	results, err := source.NewDefault().Search(ctx, query)
	if err != nil {
		return nil, err
	}
	source.RankBySeedHealth(results)
	return filterCategory(results, category), nil
}

func filterCategory(results []source.Result, category string) []source.Result {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		return results
	}
	filtered := make([]source.Result, 0, len(results))
	for _, r := range results {
		if strings.Contains(strings.ToLower(r.Category), category) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func limitResults(results []source.Result, limit int) []source.Result {
	if limit <= 0 || limit >= len(results) {
		return results
	}
	return results[:limit]
}

func writeSearchJSON(out io.Writer, results []source.Result) error {
	rows := make([]searchJSONResult, 0, len(results))
	for i, r := range results {
		rows = append(rows, searchJSONResult{
			Rank:       i + 1,
			Title:      r.Title,
			Source:     r.Source,
			SizeBytes:  r.SizeBytes,
			Popularity: r.Popularity,
			Seeders:    r.Seeders,
			Leechers:   r.Leechers,
			Files:      r.Files,
			Added:      r.Added,
			Category:   r.Category,
			TorrentURL: r.TorrentURL,
			Magnet:     r.Magnet,
		})
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func writeSearchTable(out io.Writer, results []source.Result, width int) {
	rankW := maxInt(4, len(strconv.Itoa(len(results))))
	const seedW, leechW, sizeW, sourceW = 6, 6, 9, 12
	titleW := width - rankW - seedW - leechW - sizeW - sourceW - 12
	if titleW < 20 {
		titleW = 20
	}
	fmt.Fprintf(out, "%*s  %*s  %*s  %-*s  %-*s  %s\n", rankW, "#", seedW, "Seed", leechW, "Leech", sizeW, "Size", sourceW, "Source", "Title")
	for i, r := range results {
		fmt.Fprintf(out, "%*d  %*d  %*d  %-*s  %-*s  %s\n",
			rankW, i+1,
			seedW, r.Seeders,
			leechW, r.Leechers,
			sizeW, humanBytes(r.SizeBytes),
			sourceW, truncateRunes(r.Source, sourceW),
			truncateRunes(r.Title, titleW),
		)
	}
}

func terminalWidth() int {
	if s := os.Getenv("COLUMNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 100
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:maxInt(0, n)])
	}
	return string(r[:n-1]) + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

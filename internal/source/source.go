// Package source defines where shoal finds torrents. Each provider implements
// the Source interface; the UI talks only to this interface, so adding a new
// site later (or swapping the engine) never touches the TUI.
package source

import "context"

// Result is one searchable torrent, normalized across providers.
type Result struct {
	Title      string
	Source     string // human label, e.g. "Internet Archive"
	SizeBytes  int64
	Popularity int64 // a "health" proxy: downloads, seeders, etc.
	// Category is the media type used by the UI's filter chips. For the Internet
	// Archive this is the item's mediatype ("movies", "audio", "texts",
	// "software", "image", …). Empty when the provider doesn't classify items;
	// the "All" filter ignores it.
	Category   string
	TorrentURL string // URL to a .torrent file (preferred)
	Magnet     string // optional magnet alternative
}

// Source is a searchable torrent provider.
type Source interface {
	Name() string
	Search(ctx context.Context, query string) ([]Result, error)
}

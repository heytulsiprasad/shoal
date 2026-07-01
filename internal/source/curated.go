package source

import (
	"context"
	"net/url"
	"strings"
)

// Curated is a small, built-in catalogue of canonical freely-licensed media —
// the Blender Foundation open movies (CC-BY) and the WIRED CC compilation. It
// needs no network to search (the list is compiled in) and hands the engine a
// magnet for each hit, exercising the otherwise-unused Result.Magnet path. It's
// the second legal source that proves shoal's multi-source plumbing without
// pointing at any piracy index.
//
// Infohashes are the canonical WebTorrent demo torrents (webtorrent.io/free-torrents).
type Curated struct {
	items    []curatedItem
	trackers []string
}

type curatedItem struct {
	title    string
	infohash string
	category string // Internet-Archive-style mediatype, for the UI filter chips
	keywords string // extra searchable text (not shown)
}

var openMedia = []curatedItem{
	{"Big Buck Bunny", "dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c", "movies", "blender open movie animation short comedy"},
	{"Sintel", "08ada5a7a6183aae1e09d831df6748d566095a10", "movies", "blender open movie animation fantasy"},
	{"Tears of Steel", "209c8226b299b308beaf2b9cd3fb49212dbd13ec", "movies", "blender open movie sci-fi science fiction live action"},
	{"Cosmos Laundromat", "c9e15763f722f23e98a29decdfae341b98d53056", "movies", "blender open movie animation"},
	{"The WIRED CD", "a88fda5954e89178c372716a6a78b8180ed4dad3", "audio", "creative commons music album compilation"},
}

// Public UDP trackers; anacrolix also finds peers for these well-seeded torrents
// via DHT from the infohash alone.
var defaultTrackers = []string{
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://tracker.openbittorrent.com:6969/announce",
	"udp://explodie.org:6969",
	"udp://tracker.empire-js.us:1337",
}

// NewCurated returns the built-in open-media source.
func NewCurated() *Curated {
	return &Curated{items: openMedia, trackers: defaultTrackers}
}

func (c *Curated) Name() string { return "Open Media" }

// Search returns every catalogue entry whose title/keywords/category contains
// the (case-insensitive) query; an empty query returns everything.
func (c *Curated) Search(ctx context.Context, query string) ([]Result, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]Result, 0, len(c.items))
	for _, it := range c.items {
		hay := strings.ToLower(it.title + " " + it.keywords + " " + it.category)
		if q == "" || strings.Contains(hay, q) {
			out = append(out, Result{
				Title:    it.title,
				Source:   "Open Media",
				Category: it.category,
				Magnet:   buildMagnet(it.infohash, it.title, c.trackers...),
			})
		}
	}
	return out, nil
}

// buildMagnet assembles a magnet URI from an infohash, display name and trackers.
func buildMagnet(infohash, name string, trackers ...string) string {
	if len(trackers) == 0 {
		trackers = defaultTrackers
	}
	v := url.Values{}
	v.Set("xt", "urn:btih:"+infohash)
	v.Set("dn", name)
	for _, tr := range trackers {
		v.Add("tr", tr)
	}
	return "magnet:?" + v.Encode()
}

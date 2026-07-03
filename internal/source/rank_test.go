package source

import "testing"

func TestRankBySeedHealth(t *testing.T) {
	cases := []struct {
		name string
		in   []Result
		want []string
	}{
		{
			name: "already sorted",
			in: []Result{
				{Title: "a", Seeders: 9, Leechers: 1, Popularity: 1},
				{Title: "b", Seeders: 5, Leechers: 9, Popularity: 9},
				{Title: "c", Seeders: 1, Leechers: 1, Popularity: 100},
			},
			want: []string{"a", "b", "c"},
		},
		{
			name: "needs full reorder",
			in: []Result{
				{Title: "low", Seeders: 1},
				{Title: "high", Seeders: 20},
				{Title: "mid", Seeders: 10},
			},
			want: []string{"high", "mid", "low"},
		},
		{
			name: "ties broken by leechers",
			in: []Result{
				{Title: "few", Seeders: 5, Leechers: 1},
				{Title: "many", Seeders: 5, Leechers: 9},
				{Title: "none", Seeders: 5, Leechers: 0},
			},
			want: []string{"many", "few", "none"},
		},
		{
			name: "ties broken by popularity",
			in: []Result{
				{Title: "low", Seeders: 5, Leechers: 3, Popularity: 1},
				{Title: "high", Seeders: 5, Leechers: 3, Popularity: 10},
				{Title: "mid", Seeders: 5, Leechers: 3, Popularity: 5},
			},
			want: []string{"high", "mid", "low"},
		},
		{
			name: "empty",
			in:   nil,
			want: nil,
		},
		{
			name: "single element",
			in:   []Result{{Title: "only", Seeders: 1}},
			want: []string{"only"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			RankBySeedHealth(c.in)
			if len(c.in) != len(c.want) {
				t.Fatalf("ranked %d results, want %d", len(c.in), len(c.want))
			}
			for i := range c.want {
				if c.in[i].Title != c.want[i] {
					t.Fatalf("ranked[%d] = %q, want %q; all = %+v", i, c.in[i].Title, c.want[i], c.in)
				}
			}
		})
	}
}

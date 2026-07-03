package source

import "sort"

// RankBySeedHealth orders results by swarm health. Stable so identical health
// ties preserve provider order.
func RankBySeedHealth(results []Result) {
	sort.SliceStable(results, func(a, b int) bool {
		if results[a].Seeders != results[b].Seeders {
			return results[a].Seeders > results[b].Seeders
		}
		if results[a].Leechers != results[b].Leechers {
			return results[a].Leechers > results[b].Leechers
		}
		return results[a].Popularity > results[b].Popularity
	})
}

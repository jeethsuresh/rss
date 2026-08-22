package cluster

import "sort"

func SplitComponents(members []Member, threshold float64) [][]Member {
	n := len(members)
	if n == 0 {
		return nil
	}
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if scoresAtLeast(Cosine(members[i].Vec, members[j].Vec), threshold) {
				union(i, j)
			}
		}
	}
	grouped := map[int][]Member{}
	for i, m := range members {
		r := find(i)
		grouped[r] = append(grouped[r], m)
	}
	out := make([][]Member, 0, len(grouped))
	for _, g := range grouped {
		sort.Slice(g, func(i, j int) bool { return g[i].ID < g[j].ID })
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		ai, bi := "", ""
		if len(out[i]) > 0 {
			ai = out[i][0].ID
		}
		if len(out[j]) > 0 {
			bi = out[j][0].ID
		}
		return ai < bi
	})
	return out
}

func CrossComponentOverlap(components [][]Member) []string {
	if len(components) < 2 {
		return nil
	}
	means := make([]Vector, 0, len(components))
	for _, c := range components {
		vecs := make([]Vector, 0, len(c))
		for _, m := range c {
			vecs = append(vecs, m.Vec)
		}
		means = append(means, Mean(vecs))
	}
	seen := map[string]bool{}
	for i := 0; i < len(means); i++ {
		for j := i + 1; j < len(means); j++ {
			for _, tok := range OverlapTokens(means[i], means[j]) {
				seen[tok] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for tok := range seen {
		out = append(out, tok)
	}
	sort.Strings(out)
	return out
}

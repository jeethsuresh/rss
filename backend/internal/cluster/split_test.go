package cluster

import (
	"slices"
	"testing"
)

func TestSplitComponentsSeparatesWeaklyLinkedGroups(t *testing.T) {
	army1 := Member{ID: "army1", Title: "Army", Vec: Vector{"army": 3, "gta": 1}}
	army2 := Member{ID: "army2", Title: "Reenlist", Vec: Vector{"army": 3, "reenlist": 1}}
	gta1 := Member{ID: "gta1", Title: "Leaks", Vec: Vector{"gta": 3, "leak": 1}}
	gta2 := Member{ID: "gta2", Title: "Discord", Vec: Vector{"gta": 3, "discord": 1}}
	comps := SplitComponents([]Member{army1, gta1, army2, gta2}, 0.50)
	if len(comps) != 2 {
		t.Fatalf("expected 2 components, got %d (%v)", len(comps), componentIDs(comps))
	}
	ids := componentIDs(comps)
	if !sameSet(ids[0], []string{"army1", "army2"}) && !sameSet(ids[1], []string{"army1", "army2"}) {
		t.Fatalf("army pair missing: %v", ids)
	}
	if !sameSet(ids[0], []string{"gta1", "gta2"}) && !sameSet(ids[1], []string{"gta1", "gta2"}) {
		t.Fatalf("gta pair missing: %v", ids)
	}
}

func TestSplitComponentsKeepsATightCluster(t *testing.T) {
	a := Member{ID: "a", Vec: Vector{"biden": 3, "kyiv": 3}}
	b := Member{ID: "b", Vec: Vector{"biden": 3, "kyiv": 3}}
	comps := SplitComponents([]Member{a, b}, 0.50)
	if len(comps) != 1 || len(comps[0]) != 2 {
		t.Fatalf("tight pair should stay one component, got %v", componentIDs(comps))
	}
}

func TestCrossComponentOverlapIgnoresIntraGroupTokens(t *testing.T) {
	army := []Member{
		{ID: "army1", Vec: Vector{"army": 3, "gta": 1}},
		{ID: "army2", Vec: Vector{"army": 3, "reenlist": 1}},
	}
	gta := []Member{
		{ID: "gta1", Vec: Vector{"gta": 3, "leak": 1}},
		{ID: "gta2", Vec: Vector{"gta": 3, "discord": 1}},
	}
	got := CrossComponentOverlap([][]Member{army, gta})
	if !slices.Contains(got, "gta") {
		t.Fatalf("cross-cut token gta missing: %v", got)
	}
	if slices.Contains(got, "army") || slices.Contains(got, "reenlist") || slices.Contains(got, "leak") || slices.Contains(got, "discord") {
		t.Fatalf("intra-group tokens should not be down-weighted: %v", got)
	}
}

func TestCrossComponentOverlapEmptyForOneComponent(t *testing.T) {
	got := CrossComponentOverlap([][]Member{{{ID: "a", Vec: Vector{"x": 1}}}})
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func componentIDs(comps [][]Member) [][]string {
	out := make([][]string, 0, len(comps))
	for _, c := range comps {
		ids := make([]string, 0, len(c))
		for _, m := range c {
			ids = append(ids, m.ID)
		}
		out = append(out, ids)
	}
	return out
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, id := range a {
		seen[id] = true
	}
	for _, id := range b {
		if !seen[id] {
			return false
		}
	}
	return true
}

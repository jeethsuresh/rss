package cluster

import (
	"math"
	"testing"
	"time"
)

func TestCosineIdenticalIsOne(t *testing.T) {
	v := Vector{"biden": 3, "kyiv": 3}
	got := Cosine(v, v)
	if math.Abs(got-1) > 1e-9 {
		t.Fatalf("got %v", got)
	}
}

func TestCosineEmptyIsZero(t *testing.T) {
	if Cosine(Vector{}, Vector{"a": 1}) != 0 {
		t.Fatal("empty vector should score 0")
	}
}

func TestFormatTitlePrefixesCount(t *testing.T) {
	got := FormatTitle(3, "Biden meets Zelensky")
	if got != "(3) Biden meets Zelensky" {
		t.Fatalf("got %q", got)
	}
}

func TestCentroidPicksClosestMember(t *testing.T) {
	members := []Member{
		{ID: "a", Title: "Apple event rumours swirl", Vec: Vector{"apple": 1, "event": 1}},
		{ID: "b", Title: "Apple unveils Vision Pro in Cupertino", Vec: Vector{"apple": 3, "vision": 3, "cupertino": 3}},
		{ID: "c", Title: "Analysts react to Apple hardware", Vec: Vector{"apple": 1, "analysts": 1}},
	}
	id, title := CentroidTitle(members)
	if id != "b" || title != "Apple unveils Vision Pro in Cupertino" {
		t.Fatalf("got id=%s title=%q", id, title)
	}
}

func TestOverlapTokensAreShared(t *testing.T) {
	got := OverlapTokens(Vector{"biden": 3, "kyiv": 3, "talks": 1}, Vector{"biden": 3, "nato": 3})
	if len(got) != 1 || got[0] != "biden" {
		t.Fatalf("got %v", got)
	}
}

func TestDecideCreatesPair(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	article := Candidate{ID: "a", Vec: Vector{"biden": 3, "kyiv": 3}, At: now}
	other := Candidate{ID: "b", Vec: Vector{"biden": 3, "kyiv": 3}, At: now}
	got := Decide(article, []Candidate{other}, nil, now, nil)
	if got.Action != ActionCreate {
		t.Fatalf("action %s score %v", got.Action, got.Score)
	}
	if len(got.MemberIDs) != 2 {
		t.Fatalf("members %v", got.MemberIDs)
	}
}

func TestDecideJoinsExistingStory(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	article := Candidate{ID: "a", Vec: Vector{"biden": 3, "kyiv": 3}, At: now}
	member := Candidate{ID: "b", StoryID: "s1", Vec: Vector{"biden": 3, "kyiv": 3}, At: now}
	story := StoryCandidate{ID: "s1", MemberIDs: []string{"b"}, Centroid: Vector{"biden": 3, "kyiv": 3}, Newest: now}
	got := Decide(article, []Candidate{member}, []StoryCandidate{story}, now, nil)
	if got.Action != ActionJoin || got.StoryID != "s1" {
		t.Fatalf("got %+v", got)
	}
}

func TestDecideRejectsStaleStoryAtJoinThreshold(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	article := Candidate{ID: "a", Vec: Vector{"biden": 3, "nato": 1}, At: now}
	// Similar but not massive: share one strong token among several.
	member := Candidate{ID: "b", StoryID: "s1", Vec: Vector{"biden": 3, "paris": 3, "talks": 1}, At: now.Add(-80 * time.Hour)}
	story := StoryCandidate{
		ID: "s1", MemberIDs: []string{"b"},
		Centroid: Vector{"biden": 3, "paris": 3, "talks": 1},
		Newest:   now.Add(-80 * time.Hour),
	}
	got := Decide(article, []Candidate{member}, []StoryCandidate{story}, now, nil)
	if got.Action == ActionJoin {
		t.Fatalf("stale story should reject moderate score %v", got.Score)
	}
}

func TestDecideJoinsStaleStoryAtMassiveScore(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	article := Candidate{ID: "a", Vec: Vector{"biden": 3, "kyiv": 3, "nato": 3}, At: now}
	member := Candidate{ID: "b", StoryID: "s1", Vec: Vector{"biden": 3, "kyiv": 3, "nato": 3}, At: now.Add(-80 * time.Hour)}
	story := StoryCandidate{
		ID: "s1", MemberIDs: []string{"b"},
		Centroid: Vector{"biden": 3, "kyiv": 3, "nato": 3},
		Newest:   now.Add(-80 * time.Hour),
	}
	got := Decide(article, []Candidate{member}, []StoryCandidate{story}, now, nil)
	if got.Action != ActionJoin || got.StoryID != "s1" {
		t.Fatalf("massive stale match should join, got %+v", got)
	}
	if got.Threshold != StaleJoinThreshold {
		t.Fatalf("threshold %v", got.Threshold)
	}
}

func TestDecideSkipsExcludedStory(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	article := Candidate{ID: "a", Vec: Vector{"biden": 3, "kyiv": 3}, At: now}
	member := Candidate{ID: "b", StoryID: "s1", Vec: Vector{"biden": 3, "kyiv": 3}, At: now}
	other := Candidate{ID: "c", Vec: Vector{"biden": 3, "kyiv": 3}, At: now}
	story := StoryCandidate{ID: "s1", MemberIDs: []string{"b"}, Centroid: Vector{"biden": 3, "kyiv": 3}, Newest: now}
	got := Decide(article, []Candidate{member, other}, []StoryCandidate{story}, now, map[string]bool{"s1": true})
	if got.Action != ActionCreate {
		t.Fatalf("excluded story should not be joined, got %+v", got)
	}
	if !contains(got.MemberIDs, "c") || contains(got.MemberIDs, "b") {
		t.Fatalf("should pair with ungrouped neighbour c, got %v", got.MemberIDs)
	}
}

func TestDecideIgnoresSelf(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	article := Candidate{ID: "a", Vec: Vector{"biden": 3}, At: now}
	got := Decide(article, []Candidate{article}, nil, now, nil)
	if got.Action != ActionNone {
		t.Fatalf("got %+v", got)
	}
}

func contains(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

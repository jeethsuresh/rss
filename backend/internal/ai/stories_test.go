package ai

import "testing"

func TestShouldClusterStories(t *testing.T) {
	if !ShouldClusterStories(false) {
		t.Fatal("RSS articles should be clustered")
	}
	if ShouldClusterStories(true) {
		t.Fatal("read later articles should not be clustered")
	}
}

func TestFilterStoryMemberIDsDropsReadLater(t *testing.T) {
	rl := map[string]bool{"rl": true}
	got := FilterStoryMemberIDs([]string{"a", "rl", "a", "", "b"}, func(id string) bool {
		return rl[id]
	})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %v", got)
	}
}

func TestCanCreateStoryRequiresTwoMembers(t *testing.T) {
	if CanCreateStory([]string{"only"}) {
		t.Fatal("one member must not create a story")
	}
	if !CanCreateStory([]string{"a", "b"}) {
		t.Fatal("two members should create")
	}
}

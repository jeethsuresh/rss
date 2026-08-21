package ai

// ShouldClusterStories is false for Read Later items: priority may still be set.
func ShouldClusterStories(isReadLater bool) bool {
	return !isReadLater
}

// FilterStoryMemberIDs drops empty, duplicate, and Read Later article IDs.
func FilterStoryMemberIDs(ids []string, isReadLater func(id string) bool) []string {
	out := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] || isReadLater(id) {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// CanCreateStory requires at least two eligible RSS members.
func CanCreateStory(memberIDs []string) bool {
	return len(memberIDs) >= 2
}

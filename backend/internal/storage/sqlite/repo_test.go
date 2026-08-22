package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

func TestMigrationsAndFeedCRUD(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	feeds := sqlite.NewFeedRepo(db)
	ctx := context.Background()
	now := time.Now().UTC()
	feed := &domain.Feed{
		ID:                  uuid.NewString(),
		URL:                 "https://example.com/feed.xml",
		Title:               "Example",
		PollIntervalSeconds: 3600,
		Enabled:             true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := feeds.Create(ctx, feed); err != nil {
		t.Fatal(err)
	}
	got, err := feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Example" {
		t.Fatalf("title %q", got.Title)
	}

	articles := sqlite.NewArticleRepo(db)
	a := domain.Article{
		ID:           uuid.NewString(),
		FeedID:       feed.ID,
		Title:        "Hello",
		URL:          "https://example.com/a",
		ExternalID:   "guid-1",
		DiscoveredAt: now,
	}
	n, err := articles.UpsertMany(ctx, []domain.Article{a})
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expected insert, got %d", n)
	}
	// dedupe
	a2 := a
	a2.ID = uuid.NewString()
	a2.Title = "Hello updated"
	_, err = articles.UpsertMany(ctx, []domain.Article{a2})
	if err != nil {
		t.Fatal(err)
	}
	list, err := articles.List(ctx, domain.ArticleQuery{FeedID: feed.ID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Articles) != 1 {
		t.Fatalf("expected 1 article after dedupe, got %d", len(list.Articles))
	}
}

func TestReadLaterRemoveDeletesArticle(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	feeds := sqlite.NewFeedRepo(db)
	feed, err := feeds.EnsureReadLater(ctx)
	if err != nil {
		t.Fatal(err)
	}
	articles := sqlite.NewArticleRepo(db)
	now := time.Now().UTC()
	id := uuid.NewString()
	a := domain.Article{
		ID:           id,
		FeedID:       feed.ID,
		Title:        "https://example.com/rl",
		URL:          "https://example.com/rl",
		DiscoveredAt: now,
		IsReadLater:  true,
	}
	if _, err := articles.UpsertMany(ctx, []domain.Article{a}); err != nil {
		t.Fatal(err)
	}
	if err := articles.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := articles.Get(ctx, id); err != domain.ErrNotFound {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestFolderListIncludesAssignedFeedIDs(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	feeds := sqlite.NewFeedRepo(db)
	feed := &domain.Feed{
		ID:                  uuid.NewString(),
		URL:                 "https://example.com/folder-feed.xml",
		Title:               "Folder Feed",
		PollIntervalSeconds: 3600,
		Enabled:             true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := feeds.Create(ctx, feed); err != nil {
		t.Fatal(err)
	}

	folders := sqlite.NewFolderRepo(db)
	folder := &domain.Folder{
		ID:        uuid.NewString(),
		Name:      "News",
		CreatedAt: now,
	}
	if err := folders.Create(ctx, folder); err != nil {
		t.Fatal(err)
	}
	if err := folders.AssignFeed(ctx, folder.ID, feed.ID); err != nil {
		t.Fatal(err)
	}

	list, err := folders.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(list))
	}
	if got := list[0].FeedIDs; len(got) != 1 || got[0] != feed.ID {
		t.Fatalf("expected feed %s in folder list, got %v", feed.ID, got)
	}
}

func TestFolderListIncludesMultipleAssignedFeedIDs(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	feeds := sqlite.NewFeedRepo(db)
	feedA := &domain.Feed{
		ID:                  uuid.NewString(),
		URL:                 "https://example.com/a.xml",
		Title:               "A",
		PollIntervalSeconds: 3600,
		Enabled:             true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	feedB := &domain.Feed{
		ID:                  uuid.NewString(),
		URL:                 "https://example.com/b.xml",
		Title:               "B",
		PollIntervalSeconds: 3600,
		Enabled:             true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := feeds.Create(ctx, feedA); err != nil {
		t.Fatal(err)
	}
	if err := feeds.Create(ctx, feedB); err != nil {
		t.Fatal(err)
	}

	folders := sqlite.NewFolderRepo(db)
	folder := &domain.Folder{ID: uuid.NewString(), Name: "News", CreatedAt: now}
	if err := folders.Create(ctx, folder); err != nil {
		t.Fatal(err)
	}
	if err := folders.AssignFeed(ctx, folder.ID, feedA.ID); err != nil {
		t.Fatal(err)
	}
	if err := folders.AssignFeed(ctx, folder.ID, feedB.ID); err != nil {
		t.Fatal(err)
	}

	list, err := folders.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(list))
	}
	got := list[0].FeedIDs
	if len(got) != 2 {
		t.Fatalf("expected 2 feed ids, got %v", got)
	}
	seen := map[string]bool{got[0]: true, got[1]: true}
	if !seen[feedA.ID] || !seen[feedB.ID] {
		t.Fatalf("expected feeds %s and %s, got %v", feedA.ID, feedB.ID, got)
	}
}

func TestStoryListHidesSingletonsAndReadLaterMembers(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	feeds := sqlite.NewFeedRepo(db)
	feed := &domain.Feed{
		ID:                  uuid.NewString(),
		URL:                 "https://example.com/news.xml",
		Title:               "News",
		PollIntervalSeconds: 3600,
		Enabled:             true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := feeds.Create(ctx, feed); err != nil {
		t.Fatal(err)
	}

	articles := sqlite.NewArticleRepo(db)
	rssA := domain.Article{
		ID: uuid.NewString(), FeedID: feed.ID, Title: "A", URL: "https://example.com/a",
		ExternalID: "a", DiscoveredAt: now,
	}
	rssB := domain.Article{
		ID: uuid.NewString(), FeedID: feed.ID, Title: "B", URL: "https://example.com/b",
		ExternalID: "b", DiscoveredAt: now,
	}
	rssC := domain.Article{
		ID: uuid.NewString(), FeedID: feed.ID, Title: "C", URL: "https://example.com/c",
		ExternalID: "c", DiscoveredAt: now,
	}
	rssD := domain.Article{
		ID: uuid.NewString(), FeedID: feed.ID, Title: "D", URL: "https://example.com/d",
		ExternalID: "d", DiscoveredAt: now,
	}
	rl := domain.Article{
		ID: uuid.NewString(), FeedID: feed.ID, Title: "RL", URL: "https://example.com/rl",
		ExternalID: "rl", DiscoveredAt: now, IsReadLater: true,
	}
	if _, err := articles.UpsertMany(ctx, []domain.Article{rssA, rssB, rssC, rssD, rl}); err != nil {
		t.Fatal(err)
	}

	stories := sqlite.NewStoryRepo(db)
	cluster := &domain.Story{ID: uuid.NewString(), Title: "Cluster", CreatedAt: now, UpdatedAt: now}
	singleton := &domain.Story{ID: uuid.NewString(), Title: "One", CreatedAt: now, UpdatedAt: now}
	rlPair := &domain.Story{ID: uuid.NewString(), Title: "RL pair", CreatedAt: now, UpdatedAt: now}
	for _, st := range []*domain.Story{cluster, singleton, rlPair} {
		if err := stories.Create(ctx, st); err != nil {
			t.Fatal(err)
		}
	}
	if err := stories.SetMembers(ctx, cluster.ID, []string{rssA.ID, rssB.ID}); err != nil {
		t.Fatal(err)
	}
	if err := stories.SetMembers(ctx, singleton.ID, []string{rssD.ID}); err != nil {
		t.Fatal(err)
	}
	if err := stories.SetMembers(ctx, rlPair.ID, []string{rssC.ID, rl.ID}); err != nil {
		t.Fatal(err)
	}

	list, err := stories.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != cluster.ID {
		t.Fatalf("expected only cluster story, got %+v", list)
	}
	if list[0].MemberCount != 2 {
		t.Fatalf("member count %d", list[0].MemberCount)
	}

	got, err := stories.Get(ctx, cluster.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := stories.AddMember(ctx, cluster.ID, rl.ID); err != nil {
		t.Fatal(err)
	}
	got, err = stories.Get(ctx, cluster.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.MemberCount != 2 {
		t.Fatalf("read later should not count, got %d", got.MemberCount)
	}
	for _, a := range got.Articles {
		if a.IsReadLater {
			t.Fatalf("get returned read later member %s", a.ID)
		}
	}
}

func TestStoryMarkReadDoesNotReorderList(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	feeds := sqlite.NewFeedRepo(db)
	feed := &domain.Feed{
		ID:                  uuid.NewString(),
		URL:                 "https://example.com/news.xml",
		Title:               "News",
		PollIntervalSeconds: 3600,
		Enabled:             true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := feeds.Create(ctx, feed); err != nil {
		t.Fatal(err)
	}
	articles := sqlite.NewArticleRepo(db)
	arts := make([]domain.Article, 4)
	for i := range arts {
		arts[i] = domain.Article{
			ID: uuid.NewString(), FeedID: feed.ID, Title: string(rune('A' + i)),
			URL: "https://example.com/" + string(rune('a'+i)), ExternalID: string(rune('a' + i)),
			DiscoveredAt: now,
		}
	}
	if _, err := articles.UpsertMany(ctx, arts); err != nil {
		t.Fatal(err)
	}

	stories := sqlite.NewStoryRepo(db)
	newer := &domain.Story{ID: uuid.NewString(), Title: "Newer", CreatedAt: now, UpdatedAt: now}
	older := &domain.Story{
		ID: uuid.NewString(), Title: "Older", CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	if err := stories.Create(ctx, newer); err != nil {
		t.Fatal(err)
	}
	if err := stories.Create(ctx, older); err != nil {
		t.Fatal(err)
	}
	if err := stories.SetMembers(ctx, newer.ID, []string{arts[0].ID, arts[1].ID}); err != nil {
		t.Fatal(err)
	}
	if err := stories.SetMembers(ctx, older.ID, []string{arts[2].ID, arts[3].ID}); err != nil {
		t.Fatal(err)
	}

	before, err := stories.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 2 || before[0].ID != newer.ID || before[1].ID != older.ID {
		t.Fatalf("expected newer then older, got %+v", before)
	}

	read := true
	if err := stories.CascadeFlags(ctx, older.ID, &read, nil); err != nil {
		t.Fatal(err)
	}

	after, err := stories.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 || after[0].ID != newer.ID || after[1].ID != older.ID {
		t.Fatalf("marking read must not move a story to the top, got %+v", after)
	}
	if !after[1].IsRead {
		t.Fatal("older story should be marked read")
	}
}

func TestStoryVotesAndClusteringLists(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	feeds := sqlite.NewFeedRepo(db)
	feed := &domain.Feed{
		ID: uuid.NewString(), URL: "https://example.com/feed.xml", Title: "Ex",
		PollIntervalSeconds: 3600, Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := feeds.Create(ctx, feed); err != nil {
		t.Fatal(err)
	}
	articles := sqlite.NewArticleRepo(db)
	a := domain.Article{ID: uuid.NewString(), FeedID: feed.ID, Title: "A", URL: "https://example.com/a", ExternalID: "a", DiscoveredAt: now}
	b := domain.Article{ID: uuid.NewString(), FeedID: feed.ID, Title: "B", URL: "https://example.com/b", ExternalID: "b", DiscoveredAt: now}
	if _, err := articles.UpsertMany(ctx, []domain.Article{a, b}); err != nil {
		t.Fatal(err)
	}
	stories := sqlite.NewStoryRepo(db)
	st := &domain.Story{ID: uuid.NewString(), Title: "T", Source: domain.StorySourceDeterministic, CreatedAt: now, UpdatedAt: now}
	if err := stories.Create(ctx, st); err != nil {
		t.Fatal(err)
	}
	if err := stories.SetMembers(ctx, st.ID, []string{a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	got, err := stories.Get(ctx, st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != domain.StorySourceDeterministic {
		t.Fatalf("source %q", got.Source)
	}
	if err := stories.SetArticleVote(ctx, st.ID, a.ID, domain.VoteDown, []string{a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	rec, err := stories.GetArticleVote(ctx, st.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Vote != domain.VoteDown || len(rec.Snapshot) != 2 {
		t.Fatalf("vote %+v", rec)
	}
	if err := stories.AdjustTokenWeights(ctx, []string{"biden"}, 0, 1); err != nil {
		t.Fatal(err)
	}
	weights, err := stories.GetTokenWeights(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if weights["biden"].Down != 1 {
		t.Fatalf("weights %+v", weights)
	}
	if err := stories.RemoveMember(ctx, st.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	got, err = stories.Get(ctx, st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.MemberCount != 1 {
		t.Fatalf("member count %d", got.MemberCount)
	}
	fresh, err := articles.Get(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.StoryID != "" {
		t.Fatalf("removed article still in story %s", fresh.StoryID)
	}
	listed, err := articles.ListDiscoveredSince(ctx, now.Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 {
		t.Fatalf("discovered %d", len(listed))
	}
	clustered, err := articles.ListForClustering(ctx, now.Add(-7*24*time.Hour), now.Add(-14*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(clustered) != 2 {
		t.Fatalf("clustering list %d", len(clustered))
	}
}

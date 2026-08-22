package cluster

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

func TestClusterNewGroupsSharedProperNouns(t *testing.T) {
	ctx := context.Background()
	svc, articles, stories, feedID, now := setupCluster(t)
	a := rssArticle(feedID, "Biden in Kyiv", "President Biden meets Zelensky in Kyiv after the strike.", now)
	b := rssArticle(feedID, "Talks in Kyiv", "Zelensky and Biden hold talks in Kyiv with NATO officials.", now)
	c := rssArticle(feedID, "Apple visor", "Apple unveils a new headset in Cupertino today.", now)
	if _, err := articles.UpsertMany(ctx, []domain.Article{a, b, c}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClusterNew(ctx, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	list, err := stories.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 story, got %d", len(list))
	}
	if list[0].Source != domain.StorySourceDeterministic {
		t.Fatalf("source %s", list[0].Source)
	}
	if list[0].MemberCount != 2 {
		t.Fatalf("members %d", list[0].MemberCount)
	}
	if !strings.HasPrefix(list[0].Title, "(2) ") {
		t.Fatalf("title %q", list[0].Title)
	}
	gotA, _ := articles.Get(ctx, a.ID)
	gotC, _ := articles.Get(ctx, c.ID)
	if gotA.StoryID == "" || gotA.StoryID == gotC.StoryID {
		t.Fatalf("apple article should stay ungrouped, a=%s c=%s", gotA.StoryID, gotC.StoryID)
	}
}

func TestThumbsDownReranksAndUndoRestores(t *testing.T) {
	ctx := context.Background()
	svc, articles, stories, feedID, now := setupCluster(t)
	a := rssArticle(feedID, "Biden in Kyiv", "President Biden meets Zelensky in Kyiv after the strike.", now)
	b := rssArticle(feedID, "Talks in Kyiv", "Zelensky and Biden hold talks in Kyiv with NATO officials.", now)
	if _, err := articles.UpsertMany(ctx, []domain.Article{a, b}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClusterNew(ctx, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	list, err := stories.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("setup stories %v %v", list, err)
	}
	storyID := list[0].ID
	if _, err := svc.VoteArticle(ctx, storyID, a.ID, domain.VoteDown); err != nil {
		t.Fatal(err)
	}
	list, err = stories.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("down vote should split the pair, still have %d stories", len(list))
	}
	weights, err := stories.GetTokenWeights(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(weights) == 0 {
		t.Fatal("expected overlapping tokens to be down-weighted")
	}
	if _, err := svc.VoteArticle(ctx, storyID, a.ID, domain.VoteNone); err != nil {
		t.Fatal(err)
	}
	list, err = stories.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].MemberCount != 2 {
		t.Fatalf("undo should restore cluster, got %+v", list)
	}
}

func TestSuggestIsReadOnly(t *testing.T) {
	ctx := context.Background()
	svc, articles, stories, feedID, now := setupCluster(t)
	a := rssArticle(feedID, "Biden in Kyiv", "President Biden meets Zelensky in Kyiv after the strike.", now)
	b := rssArticle(feedID, "Talks in Kyiv", "Zelensky and Biden hold talks in Kyiv with NATO officials.", now)
	if _, err := articles.UpsertMany(ctx, []domain.Article{a, b}); err != nil {
		t.Fatal(err)
	}
	sug, err := svc.Suggest(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sug.Action != ActionCreate {
		t.Fatalf("suggest action %s score %v", sug.Action, sug.Score)
	}
	list, err := stories.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatal("suggest must not write stories")
	}
}

func TestReadLaterNotClustered(t *testing.T) {
	ctx := context.Background()
	svc, articles, stories, feedID, now := setupCluster(t)
	a := rssArticle(feedID, "Biden in Kyiv", "President Biden meets Zelensky in Kyiv after the strike.", now)
	b := rssArticle(feedID, "Talks in Kyiv", "Zelensky and Biden hold talks in Kyiv with NATO officials.", now)
	b.IsReadLater = true
	if _, err := articles.UpsertMany(ctx, []domain.Article{a, b}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClusterNew(ctx, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	list, err := stories.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("read later should not cluster, got %d", len(list))
	}
}

func setupCluster(t *testing.T) (*Service, *sqlite.ArticleRepo, *sqlite.StoryRepo, string, time.Time) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
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
	stories := sqlite.NewStoryRepo(db)
	svc := New(articles, stories, nil)
	return svc, articles, stories, feed.ID, now
}

func rssArticle(feedID, title, body string, now time.Time) domain.Article {
	return domain.Article{
		ID:           uuid.NewString(),
		FeedID:       feedID,
		Title:        title,
		URL:          "https://example.com/" + uuid.NewString(),
		ExternalID:   uuid.NewString(),
		RSSContent:   body,
		Summary:      body,
		DiscoveredAt: now,
	}
}

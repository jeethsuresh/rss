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

func TestClusterPrefersExtractedBodyOverRSS(t *testing.T) {
	ctx := context.Background()
	svc, articles, stories, feedID, now := setupCluster(t)
	a := rssArticle(feedID, "Alpha headline", "unrelated rss snippet about weather", now)
	b := rssArticle(feedID, "Beta headline", "different rss snippet about sports", now)
	if _, err := articles.UpsertMany(ctx, []domain.Article{a, b}); err != nil {
		t.Fatal(err)
	}
	if err := articles.SetExtract(ctx, a.ID, "<p>President Biden meets Zelensky in Kyiv after the strike.</p>", domain.ExtractOK, "go"); err != nil {
		t.Fatal(err)
	}
	if err := articles.SetExtract(ctx, b.ID, "<p>Zelensky and Biden hold talks in Kyiv with NATO officials.</p>", domain.ExtractOK, "go"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClusterArticles(ctx, []string{a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	list, err := stories.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].MemberCount != 2 {
		t.Fatalf("extracted kyiv text should cluster, got %+v", list)
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

func TestReindexAllFixesWrongMembership(t *testing.T) {
	ctx := context.Background()
	svc, articles, stories, feedID, now := setupCluster(t)
	a := rssArticle(feedID, "Biden in Kyiv", "President Biden meets Zelensky in Kyiv after the strike.", now)
	b := rssArticle(feedID, "Talks in Kyiv", "Zelensky and Biden hold talks in Kyiv with NATO officials.", now)
	c := rssArticle(feedID, "Apple visor", "Apple unveils a new headset in Cupertino today.", now)
	if _, err := articles.UpsertMany(ctx, []domain.Article{a, b, c}); err != nil {
		t.Fatal(err)
	}
	st := &domain.Story{
		ID: uuid.NewString(), Title: "Wrong", Source: domain.StorySourceAI,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := stories.Create(ctx, st); err != nil {
		t.Fatal(err)
	}
	if err := stories.SetMembers(ctx, st.ID, []string{a.ID, b.ID, c.ID}); err != nil {
		t.Fatal(err)
	}
	n, err := svc.ReindexAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 rebuilt story, got %d", n)
	}
	list, err := stories.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d stories", len(list))
	}
	if list[0].MemberCount != 2 {
		t.Fatalf("member count %d", list[0].MemberCount)
	}
	gotC, err := articles.Get(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotC.StoryID != "" {
		t.Fatal("apple article should not remain in a meta-story")
	}
	gotA, _ := articles.Get(ctx, a.ID)
	gotB, _ := articles.Get(ctx, b.ID)
	if gotA.StoryID == "" || gotA.StoryID != gotB.StoryID {
		t.Fatalf("kyiv articles should regroup together a=%s b=%s", gotA.StoryID, gotB.StoryID)
	}
	if list[0].Source != domain.StorySourceDeterministic {
		t.Fatalf("rebuilt source %s", list[0].Source)
	}
}

func TestClusterNewWritesAILogs(t *testing.T) {
	ctx := context.Background()
	svc, articles, _, feedID, now := setupCluster(t)
	logs := &memLogs{}
	svc.Logs = logs
	a := rssArticle(feedID, "Biden in Kyiv", "President Biden meets Zelensky in Kyiv after the strike.", now)
	b := rssArticle(feedID, "Talks in Kyiv", "Zelensky and Biden hold talks in Kyiv with NATO officials.", now)
	if _, err := articles.UpsertMany(ctx, []domain.Article{a, b}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClusterNew(ctx, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if !logs.hasPrefix("deterministic index:") {
		t.Fatalf("missing index start log: %+v", logs.messages())
	}
	if !logs.hasPrefix("created meta-story") {
		t.Fatalf("missing create log: %+v", logs.messages())
	}
}

func TestReindexAllWritesAILogs(t *testing.T) {
	ctx := context.Background()
	svc, articles, _, feedID, now := setupCluster(t)
	logs := &memLogs{}
	svc.Logs = logs
	a := rssArticle(feedID, "Biden in Kyiv", "President Biden meets Zelensky in Kyiv after the strike.", now)
	b := rssArticle(feedID, "Talks in Kyiv", "Zelensky and Biden hold talks in Kyiv with NATO officials.", now)
	if _, err := articles.UpsertMany(ctx, []domain.Article{a, b}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReindexAll(ctx); err != nil {
		t.Fatal(err)
	}
	if !logs.hasPrefix("re-index started") {
		t.Fatalf("missing re-index start: %+v", logs.messages())
	}
	if !logs.hasPrefix("re-index done:") {
		t.Fatalf("missing re-index done: %+v", logs.messages())
	}
}

func TestSplitBreaksMixedMembership(t *testing.T) {
	ctx := context.Background()
	svc, articles, stories, feedID, now := setupCluster(t)
	a := rssArticle(feedID, "Biden in Kyiv", "President Biden meets Zelensky in Kyiv after the strike.", now)
	b := rssArticle(feedID, "Talks in Kyiv", "Zelensky and Biden hold talks in Kyiv with NATO officials.", now)
	c := rssArticle(feedID, "Apple visor", "Apple unveils a new headset in Cupertino today.", now)
	d := rssArticle(feedID, "Apple Cupertino", "Apple shows the headset again in Cupertino with Vision branding.", now)
	if _, err := articles.UpsertMany(ctx, []domain.Article{a, b, c, d}); err != nil {
		t.Fatal(err)
	}
	st := &domain.Story{
		ID: uuid.NewString(), Title: "(4) Mixed", Source: domain.StorySourceAI,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := stories.Create(ctx, st); err != nil {
		t.Fatal(err)
	}
	if err := stories.SetMembers(ctx, st.ID, []string{a.ID, b.ID, c.ID, d.ID}); err != nil {
		t.Fatal(err)
	}
	ids, err := svc.Split(ctx, st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 new stories, got %v", ids)
	}
	list, err := stories.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("listable %d", len(list))
	}
	gotA, _ := articles.Get(ctx, a.ID)
	gotB, _ := articles.Get(ctx, b.ID)
	gotC, _ := articles.Get(ctx, c.ID)
	gotD, _ := articles.Get(ctx, d.ID)
	if gotA.StoryID == "" || gotA.StoryID != gotB.StoryID {
		t.Fatalf("kyiv articles should stay together a=%s b=%s", gotA.StoryID, gotB.StoryID)
	}
	if gotC.StoryID == "" || gotC.StoryID != gotD.StoryID {
		t.Fatalf("apple articles should stay together c=%s d=%s", gotC.StoryID, gotD.StoryID)
	}
	if gotA.StoryID == gotC.StoryID {
		t.Fatal("mixed groups should not remain one story")
	}
	if gotA.StoryID == st.ID || gotC.StoryID == st.ID {
		t.Fatal("original story should be emptied")
	}
}

func TestSplitNoOpWhenStillOneComponent(t *testing.T) {
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
		t.Fatalf("setup %v %v", list, err)
	}
	ids, err := svc.Split(ctx, list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != list[0].ID {
		t.Fatalf("no-op should return original id, got %v", ids)
	}
	weights, err := stories.GetTokenWeights(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(weights) != 0 {
		t.Fatalf("no-op must not down-weight, got %+v", weights)
	}
	again, err := stories.List(ctx)
	if err != nil || len(again) != 1 || again[0].ID != list[0].ID || again[0].MemberCount != 2 {
		t.Fatalf("membership should be unchanged, got %+v", again)
	}
}

func TestSplitUngroupsAWeakPair(t *testing.T) {
	ctx := context.Background()
	svc, articles, stories, feedID, now := setupCluster(t)
	a := rssArticle(feedID, "Biden in Kyiv", "President Biden meets Zelensky in Kyiv after the strike.", now)
	c := rssArticle(feedID, "Apple visor", "Apple unveils a new headset in Cupertino today.", now)
	if _, err := articles.UpsertMany(ctx, []domain.Article{a, c}); err != nil {
		t.Fatal(err)
	}
	st := &domain.Story{
		ID: uuid.NewString(), Title: "(2) Mixed", Source: domain.StorySourceDeterministic,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := stories.Create(ctx, st); err != nil {
		t.Fatal(err)
	}
	if err := stories.SetMembers(ctx, st.ID, []string{a.ID, c.ID}); err != nil {
		t.Fatal(err)
	}
	ids, err := svc.Split(ctx, st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("weak pair should become singletons, got %v", ids)
	}
	list, err := stories.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("no listable stories, got %d", len(list))
	}
}

func TestSplitWritesAILog(t *testing.T) {
	ctx := context.Background()
	svc, articles, stories, feedID, now := setupCluster(t)
	logs := &memLogs{}
	svc.Logs = logs
	a := rssArticle(feedID, "Biden in Kyiv", "President Biden meets Zelensky in Kyiv after the strike.", now)
	b := rssArticle(feedID, "Talks in Kyiv", "Zelensky and Biden hold talks in Kyiv with NATO officials.", now)
	c := rssArticle(feedID, "Apple visor", "Apple unveils a new headset in Cupertino today.", now)
	d := rssArticle(feedID, "Apple Cupertino", "Apple shows the headset again in Cupertino with Vision branding.", now)
	if _, err := articles.UpsertMany(ctx, []domain.Article{a, b, c, d}); err != nil {
		t.Fatal(err)
	}
	st := &domain.Story{
		ID: uuid.NewString(), Title: "Mixed", Source: domain.StorySourceAI,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := stories.Create(ctx, st); err != nil {
		t.Fatal(err)
	}
	if err := stories.SetMembers(ctx, st.ID, []string{a.ID, b.ID, c.ID, d.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Split(ctx, st.ID); err != nil {
		t.Fatal(err)
	}
	if !logs.hasPrefix("split story:") {
		t.Fatalf("missing split log: %+v", logs.messages())
	}
}

func TestSplitDownWeightsCrossCutTokens(t *testing.T) {
	ctx := context.Background()
	svc, articles, stories, feedID, now := setupCluster(t)
	army1 := rssArticle(feedID, "Army GTA bonus", "The Army offers GTA time as a reenlistment bonus.", now)
	army2 := rssArticle(feedID, "Army reenlist GTA", "An Army unit offers days off to play GTA.", now)
	gta1 := rssArticle(feedID, "GTA leaks Discord", "Take-Two subpoenaed Discord over GTA leaks.", now)
	gta2 := rssArticle(feedID, "GTA Discord Microsoft", "Take-Two asked Discord and Microsoft about GTA leaks.", now)
	all := []domain.Article{army1, army2, gta1, gta2}
	if _, err := articles.UpsertMany(ctx, all); err != nil {
		t.Fatal(err)
	}
	st := &domain.Story{
		ID: uuid.NewString(), Title: "Mixed GTA", Source: domain.StorySourceDeterministic,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := stories.Create(ctx, st); err != nil {
		t.Fatal(err)
	}
	if err := stories.SetMembers(ctx, st.ID, []string{army1.ID, army2.ID, gta1.ID, gta2.ID}); err != nil {
		t.Fatal(err)
	}
	members := make([]Member, 0, 4)
	for _, a := range all {
		title, body := clusterText(a)
		members = append(members, Member{ID: a.ID, Title: a.Title, Vec: Tokenize(title, body, nil)})
	}
	comps := SplitComponents(members, JoinThreshold)
	overlap := CrossComponentOverlap(comps)
	if len(comps) < 2 {
		t.Fatalf("expected mixed GTA cluster to break, got %d components", len(comps))
	}
	if len(overlap) == 0 {
		t.Fatal("expected a cross-cut token such as gta")
	}
	if _, err := svc.Split(ctx, st.ID); err != nil {
		t.Fatal(err)
	}
	weights, err := stories.GetTokenWeights(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range overlap {
		if weights[tok].Down < 1 {
			t.Fatalf("cross-cut token %q should be down-weighted, got %+v", tok, weights)
		}
	}
	if fb, ok := weights["army"]; ok && fb.Down > 0 && !containsString(overlap, "army") {
		t.Fatalf("intra-group army should not be down-weighted: %+v overlap %v", fb, overlap)
	}
}

func containsString(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

type memLogs struct {
	entries []domain.AILogEntry
}

func (m *memLogs) Append(_ context.Context, entry domain.AILogEntry) error {
	m.entries = append(m.entries, entry)
	return nil
}

func (m *memLogs) List(_ context.Context, _ int) ([]domain.AILogEntry, error) {
	return m.entries, nil
}

func (m *memLogs) messages() []string {
	out := make([]string, 0, len(m.entries))
	for _, e := range m.entries {
		out = append(out, e.Message)
	}
	return out
}

func (m *memLogs) hasPrefix(prefix string) bool {
	for _, e := range m.entries {
		if strings.HasPrefix(e.Message, prefix) {
			return true
		}
	}
	return false
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

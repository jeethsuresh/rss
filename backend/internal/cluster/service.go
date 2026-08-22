package cluster

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type Service struct {
	Articles domain.ArticleRepository
	Stories  domain.StoryRepository
	Log      *slog.Logger
	Emit     func(name string, payload any)
}

func New(articles domain.ArticleRepository, stories domain.StoryRepository, log *slog.Logger) *Service {
	return &Service{Articles: articles, Stories: stories, Log: log}
}

func (s *Service) emit(name string, payload any) {
	if s.Emit != nil {
		s.Emit(name, payload)
	}
}

func (s *Service) ClusterNew(ctx context.Context, since time.Time) error {
	arts, err := s.Articles.ListDiscoveredSince(ctx, since)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(arts))
	for _, a := range arts {
		ids = append(ids, a.ID)
	}
	return s.ClusterArticles(ctx, ids)
}

func (s *Service) ClusterArticles(ctx context.Context, ids []string) error {
	for _, id := range ids {
		if err := s.clusterOne(ctx, id, nil, nil); err != nil {
			if s.Log != nil {
				s.Log.Warn("cluster article", "id", id, "err", err)
			}
		}
	}
	return nil
}

func (s *Service) Suggest(ctx context.Context, articleID string) (*Suggestion, error) {
	article, err := s.Articles.Get(ctx, articleID)
	if err != nil {
		return nil, err
	}
	if article.IsReadLater {
		return &Suggestion{Action: ActionNone}, nil
	}
	now := time.Now().UTC()
	world, err := s.loadWorld(ctx, now)
	if err != nil {
		return nil, err
	}
	cand, ok := world.byID[articleID]
	if !ok {
		cand = s.toCandidate(*article, world.weights)
	}
	sug := Decide(cand, world.articles, world.stories, now, nil)
	sug.Title = s.suggestionTitle(ctx, world, sug)
	return &sug, nil
}

func (s *Service) VoteStory(ctx context.Context, storyID string, vote domain.StoryVote) (*domain.Story, error) {
	current, err := s.Stories.GetStoryVote(ctx, storyID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if vote == domain.VoteNone || vote == current {
		if err := s.Stories.ClearStoryVote(ctx, storyID); err != nil {
			return nil, err
		}
	} else {
		if err := s.Stories.SetStoryVote(ctx, storyID, vote); err != nil {
			return nil, err
		}
	}
	s.emit("story.updated", map[string]any{"storyId": storyID})
	return s.Stories.Get(ctx, storyID)
}

func (s *Service) VoteArticle(ctx context.Context, storyID, articleID string, vote domain.StoryVote) (*domain.Story, error) {
	existing, err := s.Stories.GetArticleVote(ctx, storyID, articleID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if vote == existing.Vote || vote == domain.VoteNone {
		if existing.Vote != domain.VoteNone {
			if err := s.undoArticleVote(ctx, storyID, articleID, existing); err != nil {
				return nil, err
			}
		}
		s.emit("story.updated", map[string]any{"storyId": storyID})
		return s.Stories.Get(ctx, storyID)
	}
	if existing.Vote != domain.VoteNone {
		if err := s.undoArticleVote(ctx, storyID, articleID, existing); err != nil {
			return nil, err
		}
	}
	if err := s.applyArticleVote(ctx, storyID, articleID, vote); err != nil {
		return nil, err
	}
	s.emit("story.updated", map[string]any{"storyId": storyID})
	return s.Stories.Get(ctx, storyID)
}

type world struct {
	articles []Candidate
	stories  []StoryCandidate
	byID     map[string]Candidate
	raw      map[string]domain.Article
	weights  map[string]TokenTally
}

func (s *Service) loadWorld(ctx context.Context, now time.Time) (*world, error) {
	weights, err := s.Stories.GetTokenWeights(ctx)
	if err != nil {
		return nil, err
	}
	tallies := toTallies(weights)
	arts, err := s.Articles.ListForClustering(ctx, now.Add(-ArticleWindow), now.Add(-StoryMemberWindow))
	if err != nil {
		return nil, err
	}
	w := &world{
		articles: make([]Candidate, 0, len(arts)),
		byID:     map[string]Candidate{},
		raw:      map[string]domain.Article{},
		weights:  tallies,
	}
	members := map[string][]Candidate{}
	for _, a := range arts {
		c := s.toCandidate(a, tallies)
		w.articles = append(w.articles, c)
		w.byID[a.ID] = c
		w.raw[a.ID] = a
		if a.StoryID != "" {
			members[a.StoryID] = append(members[a.StoryID], c)
		}
	}
	for storyID, mems := range members {
		vecs := make([]Vector, 0, len(mems))
		newest := time.Time{}
		ids := make([]string, 0, len(mems))
		for _, m := range mems {
			vecs = append(vecs, m.Vec)
			ids = append(ids, m.ID)
			if m.At.After(newest) {
				newest = m.At
			}
		}
		w.stories = append(w.stories, StoryCandidate{
			ID: storyID, MemberIDs: ids, Centroid: Mean(vecs), Newest: newest,
		})
	}
	return w, nil
}

func (s *Service) toCandidate(a domain.Article, weights map[string]TokenTally) Candidate {
	title, body := rssText(a)
	return Candidate{
		ID:      a.ID,
		StoryID: a.StoryID,
		Vec:     Tokenize(title, body, weights),
		At:      articleTime(a),
	}
}

func (s *Service) clusterOne(ctx context.Context, articleID string, excludeStory, excludeArticle map[string]bool) error {
	article, err := s.Articles.Get(ctx, articleID)
	if err != nil {
		return err
	}
	if !shouldCluster(*article) {
		return nil
	}
	if article.StoryID != "" && !excludeStory[article.StoryID] {
		return nil
	}
	now := time.Now().UTC()
	world, err := s.loadWorld(ctx, now)
	if err != nil {
		return err
	}
	cand, ok := world.byID[articleID]
	if !ok {
		cand = s.toCandidate(*article, world.weights)
	}
	if excludeStory[cand.StoryID] {
		cand.StoryID = ""
	}
	sug := DecideExcluding(cand, world.articles, world.stories, now, excludeStory, excludeArticle)
	return s.applySuggestion(ctx, article, sug)
}

func (s *Service) applySuggestion(ctx context.Context, article *domain.Article, sug Suggestion) error {
	switch sug.Action {
	case ActionJoin:
		if sug.StoryID == "" {
			return nil
		}
		if err := s.Stories.AddMember(ctx, sug.StoryID, article.ID); err != nil {
			return err
		}
		if err := s.refreshDeterministicTitle(ctx, sug.StoryID); err != nil {
			return err
		}
		s.emit("story.updated", map[string]any{"storyId": sug.StoryID})
		return nil
	case ActionCreate:
		members := uniqueIDs(sug.MemberIDs)
		if len(members) < 2 {
			return nil
		}
		now := time.Now().UTC()
		title, summary := s.deterministicTitle(ctx, members)
		st := &domain.Story{
			ID:        uuid.NewString(),
			Title:     title,
			Summary:   summary,
			Source:    domain.StorySourceDeterministic,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.Stories.Create(ctx, st); err != nil {
			return err
		}
		if err := s.Stories.SetMembers(ctx, st.ID, members); err != nil {
			return err
		}
		s.emit("story.updated", map[string]any{"storyId": st.ID})
		return nil
	default:
		return nil
	}
}

func (s *Service) applyArticleVote(ctx context.Context, storyID, articleID string, vote domain.StoryVote) error {
	st, err := s.Stories.Get(ctx, storyID)
	if err != nil {
		return err
	}
	snapshot := append([]string{}, st.ArticleIDs...)
	tokens := s.overlapTokens(st, articleID)
	switch vote {
	case domain.VoteUp:
		if err := s.Stories.AdjustTokenWeights(ctx, tokens, 1, 0); err != nil {
			return err
		}
		return s.Stories.SetArticleVote(ctx, storyID, articleID, vote, snapshot)
	case domain.VoteDown:
		if err := s.Stories.AdjustTokenWeights(ctx, tokens, 0, 1); err != nil {
			return err
		}
		if err := s.Stories.SetArticleVote(ctx, storyID, articleID, vote, snapshot); err != nil {
			return err
		}
		if err := s.Stories.RemoveMember(ctx, storyID, articleID); err != nil {
			return err
		}
		if err := s.refreshDeterministicTitle(ctx, storyID); err != nil {
			return err
		}
		excludeStory := map[string]bool{storyID: true}
		if err := s.clusterOne(ctx, articleID, excludeStory, siblingIDs(snapshot, articleID)); err != nil {
			return err
		}
		return s.rerankLeftover(ctx, storyID, map[string]bool{articleID: true})
	default:
		return nil
	}
}

func siblingIDs(snapshot []string, articleID string) map[string]bool {
	out := map[string]bool{}
	for _, id := range snapshot {
		if id != articleID {
			out[id] = true
		}
	}
	return out
}

func (s *Service) undoArticleVote(ctx context.Context, storyID, articleID string, rec domain.ArticleVoteRecord) error {
	st, err := s.Stories.Get(ctx, storyID)
	if err != nil {
		return err
	}
	tokens := s.overlapFromSnapshot(ctx, rec.Snapshot, articleID)
	if tokens == nil {
		tokens = s.overlapTokens(st, articleID)
	}
	switch rec.Vote {
	case domain.VoteUp:
		if err := s.Stories.AdjustTokenWeights(ctx, tokens, -1, 0); err != nil {
			return err
		}
	case domain.VoteDown:
		if err := s.Stories.AdjustTokenWeights(ctx, tokens, 0, -1); err != nil {
			return err
		}
		if len(rec.Snapshot) > 0 {
			if err := s.Stories.SetMembers(ctx, storyID, rec.Snapshot); err != nil {
				return err
			}
			if err := s.refreshDeterministicTitle(ctx, storyID); err != nil {
				return err
			}
		}
	}
	if err := s.Stories.ClearArticleVote(ctx, storyID, articleID); err != nil {
		return err
	}
	s.emit("story.updated", map[string]any{"storyId": storyID})
	return nil
}

func (s *Service) rerankLeftover(ctx context.Context, storyID string, excludeArticle map[string]bool) error {
	st, err := s.Stories.Get(ctx, storyID)
	if err != nil {
		return err
	}
	ids := st.ArticleIDs
	if len(ids) == 0 {
		return nil
	}
	if len(ids) == 1 {
		return s.clusterOne(ctx, ids[0], map[string]bool{storyID: true}, excludeArticle)
	}
	now := time.Now().UTC()
	world, err := s.loadWorld(ctx, now)
	if err != nil {
		return err
	}
	vecs := make([]Vector, 0, len(ids))
	for _, id := range ids {
		if c, ok := world.byID[id]; ok {
			vecs = append(vecs, c.Vec)
		}
	}
	centroid := Mean(vecs)
	target, score, _ := bestStoryForCentroid(centroid, world.stories, now, map[string]bool{storyID: true})
	if target == "" || score < JoinThreshold {
		return nil
	}
	for _, id := range ids {
		if err := s.Stories.AddMember(ctx, target, id); err != nil {
			return err
		}
	}
	if err := s.refreshDeterministicTitle(ctx, target); err != nil {
		return err
	}
	s.emit("story.updated", map[string]any{"storyId": target})
	s.emit("story.updated", map[string]any{"storyId": storyID})
	return nil
}

func bestStoryForCentroid(centroid Vector, stories []StoryCandidate, now time.Time, exclude map[string]bool) (storyID string, score, thresh float64) {
	best := -1.0
	for _, st := range stories {
		if exclude[st.ID] {
			continue
		}
		sc := Cosine(centroid, st.Centroid)
		th := JoinThreshold
		if now.Sub(st.Newest) > StaleAge {
			th = StaleJoinThreshold
		}
		if sc >= th && sc > best {
			best = sc
			storyID = st.ID
			score = sc
			thresh = th
		}
	}
	return storyID, score, thresh
}

func (s *Service) overlapTokens(st *domain.Story, articleID string) []string {
	var article domain.Article
	rest := Vector{}
	weights := map[string]TokenTally{}
	for _, a := range st.Articles {
		title, body := rssText(a)
		vec := Tokenize(title, body, weights)
		if a.ID == articleID {
			article = a
			continue
		}
		for k, v := range vec {
			rest[k] += v
		}
	}
	if article.ID == "" {
		return nil
	}
	title, body := rssText(article)
	return OverlapTokens(Tokenize(title, body, weights), rest)
}

func (s *Service) overlapFromSnapshot(ctx context.Context, snapshot []string, articleID string) []string {
	if len(snapshot) == 0 {
		return nil
	}
	var article *domain.Article
	rest := Vector{}
	for _, id := range snapshot {
		a, err := s.Articles.Get(ctx, id)
		if err != nil {
			continue
		}
		title, body := rssText(*a)
		vec := Tokenize(title, body, nil)
		if id == articleID {
			article = a
			continue
		}
		for k, v := range vec {
			rest[k] += v
		}
	}
	if article == nil {
		return nil
	}
	title, body := rssText(*article)
	return OverlapTokens(Tokenize(title, body, nil), rest)
}

func (s *Service) refreshDeterministicTitle(ctx context.Context, storyID string) error {
	st, err := s.Stories.Get(ctx, storyID)
	if err != nil {
		return err
	}
	if st.Source != domain.StorySourceDeterministic {
		return nil
	}
	if len(st.ArticleIDs) < 2 {
		return nil
	}
	title, summary := s.deterministicTitle(ctx, st.ArticleIDs)
	st.Title = title
	st.Summary = summary
	st.UpdatedAt = time.Now().UTC()
	return s.Stories.Update(ctx, st)
}

func (s *Service) deterministicTitle(ctx context.Context, memberIDs []string) (string, string) {
	members := make([]Member, 0, len(memberIDs))
	summaries := map[string]string{}
	for _, id := range memberIDs {
		a, err := s.Articles.Get(ctx, id)
		if err != nil || a.IsReadLater {
			continue
		}
		title, body := rssText(*a)
		members = append(members, Member{ID: a.ID, Title: a.Title, Vec: Tokenize(title, body, nil)})
		sum := a.Summary
		if sum == "" {
			sum = body
		}
		summaries[a.ID] = truncateText(stripTags(sum), 280)
	}
	id, title := CentroidTitle(members)
	return FormatTitle(len(members), title), summaries[id]
}

func (s *Service) suggestionTitle(ctx context.Context, world *world, sug Suggestion) string {
	switch sug.Action {
	case ActionJoin:
		if st, err := s.Stories.Get(ctx, sug.StoryID); err == nil {
			return st.Title
		}
	case ActionCreate:
		title, _ := s.deterministicTitle(ctx, sug.MemberIDs)
		return title
	}
	return ""
}

func shouldCluster(a domain.Article) bool {
	return !a.IsReadLater
}

func rssText(a domain.Article) (title, body string) {
	body = a.RSSContent
	if body == "" {
		body = a.Summary
	}
	return a.Title, body
}

func articleTime(a domain.Article) time.Time {
	if a.PublishedAt != nil {
		return a.PublishedAt.UTC()
	}
	return a.DiscoveredAt.UTC()
}

func toTallies(in map[string]domain.TokenFeedback) map[string]TokenTally {
	out := map[string]TokenTally{}
	for k, v := range in {
		out[k] = TokenTally{Up: v.Up, Down: v.Down}
	}
	return out
}

func uniqueIDs(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

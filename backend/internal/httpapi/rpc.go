package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
)

type rpcRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func (a *API) rpc(w http.ResponseWriter, r *http.Request) {
	var request rpcRequest
	if err := decodeJSON(r, &request); err != nil || strings.TrimSpace(request.Method) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", errors.New("method is required"))
		return
	}
	if len(request.Params) == 0 {
		request.Params = json.RawMessage(`{}`)
	}
	result, err := a.dispatchRPC(r.Context(), currentUser(r.Context()).ID, request)
	respond(w, result, err)
}

func rpcParams[T any](raw json.RawMessage) (T, error) {
	var params T
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return params, domain.ErrInvalidParams
	}
	return params, nil
}

func (a *API) dispatchRPC(ctx context.Context, userID string, request rpcRequest) (any, error) {
	switch request.Method {
	case "system.ping":
		return map[string]any{"ok": true, "version": a.cfg.Version}, nil
	case "system.info":
		return map[string]any{"version": a.cfg.Version, "dbPath": "", "protocolVersion": 1}, nil
	case "feeds.list":
		return a.store.ListWebFeeds(ctx, userID)
	case "feeds.get":
		params, err := rpcParams[struct {
			ID string `json:"id"`
		}](request.Params)
		if err != nil || params.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return a.store.GetUserFeed(ctx, userID, params.ID)
	case "feeds.preview":
		params, err := rpcParams[struct {
			URL string `json:"url"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		feedURL, err := serverstore.NormalizeURL(params.URL)
		if err != nil {
			return nil, err
		}
		return domain.FeedPreview{URL: feedURL, Title: feedURL}, nil
	case "feeds.add":
		params, err := rpcParams[struct {
			URL string `json:"url"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		return a.store.SetFeedMembership(ctx, userID, params.URL, true)
	case "feeds.remove":
		params, err := rpcParams[struct {
			ID string `json:"id"`
		}](request.Params)
		if err != nil || params.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return nil, a.store.RemoveFeedMembership(ctx, userID, params.ID)
	case "feeds.refresh":
		params, err := rpcParams[struct {
			ID string `json:"id"`
		}](request.Params)
		if err != nil || params.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return nil, a.store.QueueFeedRefresh(ctx, userID, params.ID)
	case "feeds.refreshAll":
		return nil, a.store.QueueAllFeedRefreshes(ctx, userID)
	case "feeds.setEnabled":
		params, err := rpcParams[struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
		}](request.Params)
		if err != nil || params.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return a.store.SetUserFeedEnabled(ctx, userID, params.ID, params.Enabled)
	case "feeds.setPollInterval":
		params, err := rpcParams[struct {
			ID      string `json:"id"`
			Seconds int    `json:"seconds"`
		}](request.Params)
		if err != nil || params.ID == "" {
			return nil, domain.ErrInvalidParams
		}
		return a.store.SetUserFeedPollInterval(ctx, userID, params.ID, params.Seconds)
	case "feeds.exportUrls":
		feeds, err := a.store.ListFeeds(ctx, userID)
		if err != nil {
			return nil, err
		}
		urls := make([]string, 0, len(feeds))
		for _, feed := range feeds {
			urls = append(urls, feed.URL)
		}
		sort.Strings(urls)
		return map[string]any{"text": strings.Join(urls, "\n")}, nil
	case "feeds.importUrls":
		params, err := rpcParams[struct {
			Text string `json:"text"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		result := map[string]any{"added": 0, "failed": 0, "errors": []string{}}
		errorsList := []string{}
		added, failed := 0, 0
		seen := map[string]bool{}
		for _, line := range strings.Fields(params.Text) {
			url, err := serverstore.NormalizeURL(line)
			if err != nil || seen[url] {
				if err != nil {
					failed++
					errorsList = append(errorsList, line)
				}
				continue
			}
			seen[url] = true
			if _, err := a.store.SetFeedMembership(ctx, userID, url, true); err != nil {
				failed++
				errorsList = append(errorsList, line)
			} else {
				added++
			}
		}
		result["added"], result["failed"], result["errors"] = added, failed, errorsList
		return result, nil
	case "articles.list":
		params, err := rpcParams[domain.ArticleQuery](request.Params)
		if err != nil {
			return nil, err
		}
		return a.store.ListArticlesPage(ctx, userID, params)
	case "articles.get":
		params, err := articleIDParams(request.Params)
		if err != nil {
			return nil, err
		}
		return a.webArticle(ctx, userID, params)
	case "articles.markRead", "articles.markUnread":
		id, err := articleIDParams(request.Params)
		if err != nil {
			return nil, err
		}
		value := request.Method == "articles.markRead"
		if _, err := a.store.ReadLaterArticle(ctx, userID, id); err == nil {
			return a.store.SetReadLaterArticleState(ctx, userID, id, &value, nil)
		}
		return a.store.SetArticleState(ctx, userID, id, serverstore.ArticleStatePatch{IsRead: &value})
	case "articles.toggleStar":
		id, err := articleIDParams(request.Params)
		if err != nil {
			return nil, err
		}
		if item, err := a.store.ReadLaterArticle(ctx, userID, id); err == nil {
			value := !item.IsStarred
			return a.store.SetReadLaterArticleState(ctx, userID, id, nil, &value)
		}
		article, err := a.store.GetArticle(ctx, userID, id)
		if err != nil {
			return nil, err
		}
		value := !article.IsStarred
		return a.store.SetArticleState(ctx, userID, id, serverstore.ArticleStatePatch{IsStarred: &value})
	case "articles.markAllRead":
		params, err := rpcParams[domain.ArticleQuery](request.Params)
		if err != nil {
			return nil, err
		}
		updated, err := a.store.MarkAllArticlesRead(ctx, userID, params)
		return map[string]any{"updated": updated}, err
	case "articles.recrawl":
		id, err := articleIDParams(request.Params)
		if err != nil {
			return nil, err
		}
		if _, err := a.store.ReadLaterArticle(ctx, userID, id); err == nil {
			return a.store.QueueReadLaterRefresh(ctx, userID, id)
		}
		article, err := a.store.QueueArticleRefresh(ctx, userID, id)
		if err == nil && a.svc.Crawler != nil {
			a.svc.Crawler.EnqueueAndKick(a.ctx)
		}
		return article, err
	case "articles.fetchLive", "articles.setExtract":
		id, err := articleIDParams(request.Params)
		if err != nil {
			return nil, err
		}
		return a.webArticle(ctx, userID, id)
	case "articles.pendingExtract":
		return map[string]any{"articleIds": []string{}}, nil
	case "readLater.add":
		params, err := rpcParams[struct {
			URL string `json:"url"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		item, err := a.store.AddReadLater(ctx, userID, params.URL)
		if err != nil {
			return nil, err
		}
		return a.store.ReadLaterArticle(ctx, userID, item.ID)
	case "readLater.addFromArticle":
		params, err := rpcParams[struct {
			ArticleID string `json:"articleId"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		article, err := a.store.GetArticle(ctx, userID, params.ArticleID)
		if err != nil {
			return nil, err
		}
		item, err := a.store.AddReadLater(ctx, userID, article.URL)
		if err != nil {
			return nil, err
		}
		return a.store.ReadLaterArticle(ctx, userID, item.ID)
	case "readLater.list":
		params, err := rpcParams[struct {
			Filter string `json:"filter"`
			Search string `json:"search"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		return a.store.ListReadLaterArticles(ctx, userID, params.Filter, params.Search)
	case "readLater.archive", "readLater.unarchive":
		id, err := articleIDParams(request.Params)
		if err != nil {
			return nil, err
		}
		return a.store.SetReadLaterArchived(ctx, userID, id, request.Method == "readLater.archive")
	case "readLater.remove":
		id, err := articleIDParams(request.Params)
		if err != nil {
			return nil, err
		}
		return nil, a.store.DeleteReadLater(ctx, userID, id)
	case "sports.teams.list":
		return a.svc.SportsTeams(ctx)
	case "sports.seasons.list":
		return a.svc.SportsSeasons(ctx)
	case "sports.followed.get":
		return followedTeamInts(a.store.FollowedTeams(ctx, userID, "mlb"))
	case "sports.followed.set":
		params, err := rpcParams[struct {
			TeamIDs []int `json:"teamIds"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(params.TeamIDs))
		for _, id := range params.TeamIDs {
			ids = append(ids, strconv.Itoa(id))
		}
		return followedTeamInts(a.store.SetFollowedTeams(ctx, userID, "mlb", ids))
	case "sports.followed.toggle":
		params, err := rpcParams[struct {
			TeamID int `json:"teamId"`
		}](request.Params)
		if err != nil || params.TeamID <= 0 {
			return nil, domain.ErrInvalidParams
		}
		current, err := a.store.FollowedTeams(ctx, userID, "mlb")
		if err != nil {
			return nil, err
		}
		target := strconv.Itoa(params.TeamID)
		next := []string{}
		found := false
		for _, id := range current {
			if id == target {
				found = true
				continue
			}
			next = append(next, id)
		}
		if !found {
			next = append(next, target)
		}
		return followedTeamInts(a.store.SetFollowedTeams(ctx, userID, "mlb", next))
	case "sports.schedule.list":
		params, err := rpcParams[struct {
			TeamID int `json:"teamId"`
			Season int `json:"season"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		return a.svc.SportsSchedule(ctx, params.TeamID, params.Season)
	case "sports.schedule.daily":
		params, err := rpcParams[struct {
			Date string `json:"date"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		return a.svc.SportsDailySchedule(ctx, params.Date)
	case "sports.game.get", "sports.game.watch":
		params, err := rpcParams[struct {
			GamePk int `json:"gamePk"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		if request.Method == "sports.game.watch" {
			return a.svc.SportsGameWatch(ctx, params.GamePk)
		}
		return a.svc.SportsGameGet(ctx, params.GamePk)
	case "sports.game.unwatch":
		params, err := rpcParams[struct {
			GamePk int `json:"gamePk"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		err = a.svc.SportsGameUnwatch(ctx, params.GamePk)
		return map[string]any{"ok": err == nil}, err
	case "sports.standings.get":
		params, err := rpcParams[struct {
			Season int `json:"season"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		return a.svc.SportsStandings(ctx, params.Season)
	case "sports.roster.get":
		params, err := rpcParams[struct {
			TeamID int `json:"teamId"`
			Season int `json:"season"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		return a.svc.SportsRoster(ctx, params.TeamID, params.Season)
	case "sports.f1.years.list":
		return a.svc.SportsF1Years(ctx)
	case "sports.f1.races.list":
		params, err := rpcParams[struct {
			Year int `json:"year"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		return a.svc.SportsF1Races(ctx, params.Year)
	case "sports.f1.race.get", "sports.f1.race.watch":
		params, err := rpcParams[struct {
			SessionKey int `json:"sessionKey"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		if request.Method == "sports.f1.race.watch" {
			return a.svc.SportsF1RaceWatch(ctx, params.SessionKey)
		}
		return a.svc.SportsF1RaceGet(ctx, params.SessionKey)
	case "sports.f1.race.unwatch":
		params, err := rpcParams[struct {
			SessionKey int `json:"sessionKey"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		err = a.svc.SportsF1RaceUnwatch(ctx, params.SessionKey)
		return map[string]any{"ok": err == nil}, err
	case "sports.f1.standings.get":
		params, err := rpcParams[struct {
			Year int `json:"year"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		return a.svc.SportsF1Standings(ctx, params.Year)
	case "stories.list":
		return a.store.ListStories(ctx, userID)
	case "stories.get":
		id, err := storyIDParams(request.Params)
		if err != nil {
			return nil, err
		}
		return a.store.GetStory(ctx, userID, id)
	case "stories.markRead", "stories.markUnread":
		id, err := storyIDParams(request.Params)
		if err != nil {
			return nil, err
		}
		value := request.Method == "stories.markRead"
		if err := a.store.SetStoryState(ctx, userID, id, serverstore.ArticleStatePatch{IsRead: &value}); err != nil {
			return nil, err
		}
		return a.store.GetStory(ctx, userID, id)
	case "stories.toggleStar":
		id, err := storyIDParams(request.Params)
		if err != nil {
			return nil, err
		}
		story, err := a.store.GetStory(ctx, userID, id)
		if err != nil {
			return nil, err
		}
		value := !story.IsStarred
		if err := a.store.SetStoryState(ctx, userID, id, serverstore.ArticleStatePatch{IsStarred: &value}); err != nil {
			return nil, err
		}
		return a.store.GetStory(ctx, userID, id)
	case "stories.voteArticle":
		params, err := rpcParams[struct {
			StoryID   string `json:"storyId"`
			ArticleID string `json:"articleId"`
			Vote      string `json:"vote"`
		}](request.Params)
		if err != nil || !validVote(params.Vote) {
			return nil, domain.ErrInvalidParams
		}
		if _, err := a.store.GetStory(ctx, userID, params.StoryID); err != nil {
			return nil, err
		}
		if _, err := a.svc.VoteStoryArticle(ctx, params.StoryID, params.ArticleID, rpcStoryVote(params.Vote)); err != nil {
			return nil, err
		}
		return a.store.GetStory(ctx, userID, params.StoryID)
	case "stories.voteStory":
		params, err := rpcParams[struct {
			ID   string `json:"id"`
			Vote string `json:"vote"`
		}](request.Params)
		if err != nil || !validVote(params.Vote) {
			return nil, domain.ErrInvalidParams
		}
		if _, err := a.store.GetStory(ctx, userID, params.ID); err != nil {
			return nil, err
		}
		if _, err := a.svc.VoteStory(ctx, params.ID, rpcStoryVote(params.Vote)); err != nil {
			return nil, err
		}
		return a.store.GetStory(ctx, userID, params.ID)
	case "stories.reindex":
		count, err := a.svc.ReindexStories(ctx)
		return map[string]any{"storyCount": count}, err
	case "stories.split":
		id, err := storyIDParams(request.Params)
		if err != nil {
			return nil, err
		}
		if _, err := a.store.GetStory(ctx, userID, id); err != nil {
			return nil, err
		}
		ids, err := a.svc.SplitStory(ctx, id)
		return map[string]any{"storyIds": ids}, err
	case "folders.list":
		return a.store.ListFolders(ctx, userID)
	case "folders.create":
		params, err := rpcParams[struct {
			Name string `json:"name"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		return a.store.CreateFolder(ctx, userID, params.Name)
	case "folders.remove":
		id, err := folderIDParams(request.Params)
		if err != nil {
			return nil, err
		}
		return nil, a.store.DeleteFolder(ctx, userID, id)
	case "folders.assignFeed", "folders.unassignFeed":
		params, err := rpcParams[struct {
			FolderID string `json:"folderId"`
			FeedID   string `json:"feedId"`
		}](request.Params)
		if err != nil {
			return nil, err
		}
		return nil, a.store.AssignFolder(ctx, userID, params.FolderID, params.FeedID, request.Method == "folders.assignFeed")
	case "settings.get":
		return a.webSettings(ctx, userID, nil)
	case "settings.update":
		params, err := rpcParams[map[string]any](request.Params)
		if err != nil {
			return nil, err
		}
		delete(params, "aiEnabled")
		delete(params, "aiBaseUrl")
		delete(params, "aiModel")
		settings, err := a.store.UpdateSettings(ctx, userID, params)
		return a.webSettings(ctx, userID, settings, err)
	case "ai.test":
		if a.svc.AI == nil {
			return nil, domain.ErrInvalidParams
		}
		return a.svc.AI.Test(ctx)
	case "ai.scan":
		params, err := rpcParams[struct {
			Window string `json:"window"`
		}](request.Params)
		if err != nil || a.svc.AI == nil {
			return nil, domain.ErrInvalidParams
		}
		err = a.svc.AI.ScanWindow(ctx, params.Window)
		return map[string]any{"queued": err == nil, "status": a.svc.AI.Status(ctx)}, err
	case "ai.status":
		if a.svc.AI == nil {
			return domain.AIStatus{}, nil
		}
		return a.svc.AI.Status(ctx), nil
	case "ai.logs":
		params, _ := rpcParams[struct {
			Limit int `json:"limit"`
		}](request.Params)
		if a.svc.AI == nil {
			return []domain.AILogEntry{}, nil
		}
		return a.store.ListAILogs(ctx, userID, params.Limit)
	case "ai.retryFailed":
		if a.svc.AI == nil {
			return nil, domain.ErrInvalidParams
		}
		count, err := a.svc.AI.RetryFailed(ctx)
		return map[string]any{"requeued": count, "status": a.svc.AI.Status(ctx)}, err
	case "errors.list":
		// Error logs may contain details from another tenant's background work.
		return []domain.ErrorLogEntry{}, nil
	default:
		return nil, errors.New("unsupported method")
	}
}

func (a *API) webArticle(ctx context.Context, userID, id string) (*domain.Article, error) {
	article, err := a.store.GetArticle(ctx, userID, id)
	if err == nil {
		return article, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	return a.store.ReadLaterArticle(ctx, userID, id)
}

func articleIDParams(raw json.RawMessage) (string, error) {
	params, err := rpcParams[struct {
		ID        string `json:"id"`
		ArticleID string `json:"articleId"`
	}](raw)
	if err != nil {
		return "", err
	}
	if params.ID != "" {
		return params.ID, nil
	}
	if params.ArticleID != "" {
		return params.ArticleID, nil
	}
	return "", domain.ErrInvalidParams
}

func storyIDParams(raw json.RawMessage) (string, error) {
	params, err := rpcParams[struct {
		ID string `json:"id"`
	}](raw)
	if err != nil || params.ID == "" {
		return "", domain.ErrInvalidParams
	}
	return params.ID, nil
}

func folderIDParams(raw json.RawMessage) (string, error) {
	return storyIDParams(raw)
}

func followedTeamInts(ids []string, err error) ([]int, error) {
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		value, err := strconv.Atoi(id)
		if err == nil {
			out = append(out, value)
		}
	}
	return out, nil
}

func validVote(vote string) bool {
	return vote == "" || vote == "none" || vote == "up" || vote == "down"
}

func rpcStoryVote(vote string) domain.StoryVote {
	if vote == "none" {
		return domain.VoteNone
	}
	return domain.StoryVote(vote)
}

func (a *API) webSettings(ctx context.Context, userID string, current *domain.Settings, priorErr ...error) (*domain.Settings, error) {
	if len(priorErr) > 0 && priorErr[0] != nil {
		return nil, priorErr[0]
	}
	var err error
	if current == nil {
		current, err = a.store.GetSettings(ctx, userID)
		if err != nil {
			return nil, err
		}
	}
	if a.svc.Settings != nil {
		global, err := a.svc.Settings.Get(ctx)
		if err == nil {
			current.AIEnabled = global.AIEnabled
			current.AIBaseURL = global.AIBaseURL
			current.AIModel = global.AIModel
		}
	}
	return current, nil
}

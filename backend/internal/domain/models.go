package domain

import "time"

type Priority string

const (
	PriorityNone   Priority = "none"
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

type CrawlStatus string

const (
	CrawlNone    CrawlStatus = "none"
	CrawlPending CrawlStatus = "pending"
	CrawlOK      CrawlStatus = "ok"
	CrawlFailed  CrawlStatus = "failed"
)

type Feed struct {
	ID                  string     `json:"id"`
	URL                 string     `json:"url"`
	Title               string     `json:"title"`
	Description         string     `json:"description"`
	SiteURL             string     `json:"siteUrl"`
	IconURL             string     `json:"iconUrl"`
	LastSuccessAt       *time.Time `json:"lastSuccessAt"`
	LastAttemptAt       *time.Time `json:"lastAttemptAt"`
	LastError           string     `json:"lastError"`
	ETag                string     `json:"etag"`
	LastModified        string     `json:"lastModified"`
	PollIntervalSeconds int        `json:"pollIntervalSeconds"`
	Enabled             bool       `json:"enabled"`
	UnreadCount         int        `json:"unreadCount"`
	IsReadLater         bool       `json:"isReadLater"`
	CrawlAttempts       int        `json:"crawlAttempts"`
	CrawlFailures       int        `json:"crawlFailures"`
	BadCrawlPercent     float64    `json:"badCrawlPercent"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type Article struct {
	ID              string      `json:"id"`
	FeedID          string      `json:"feedId"`
	Title           string      `json:"title"`
	URL             string      `json:"url"`
	Author          string      `json:"author"`
	Content         string      `json:"content"` // active display helper = rss or crawled preferred
	Summary         string      `json:"summary"`
	RSSContent      string      `json:"rssContent"`
	CrawledContent  string      `json:"crawledContent"`
	LiveContent     string      `json:"liveContent"`
	CrawlStatus     CrawlStatus `json:"crawlStatus"`
	CrawlError      string      `json:"crawlError"`
	CrawlUnreliable bool        `json:"crawlUnreliable"`
	IsReadLater     bool        `json:"isReadLater"`
	ArchivedAt      *time.Time  `json:"archivedAt,omitempty"`
	PublishedAt     *time.Time  `json:"publishedAt"`
	UpdatedAt       *time.Time  `json:"updatedAt"`
	ExternalID      string      `json:"externalId"`
	IsRead          bool        `json:"isRead"`
	IsStarred       bool        `json:"isStarred"`
	Priority        Priority    `json:"priority"`
	StoryID         string      `json:"storyId,omitempty"`
	DiscoveredAt    time.Time   `json:"discoveredAt"`
	FeedTitle       string      `json:"feedTitle,omitempty"`
}

const (
	StorySourceAI            = "ai"
	StorySourceDeterministic = "deterministic"
)

type StoryVote string

const (
	VoteNone StoryVote = ""
	VoteUp   StoryVote = "up"
	VoteDown StoryVote = "down"
)

type TokenFeedback struct {
	Up   int
	Down int
}

type ArticleVoteRecord struct {
	Vote     StoryVote
	Snapshot []string
}

type Story struct {
	ID           string               `json:"id"`
	Title        string               `json:"title"`
	Summary      string               `json:"summary"`
	Source       string               `json:"source"`
	Vote         StoryVote            `json:"vote,omitempty"`
	ArticleVotes map[string]StoryVote `json:"articleVotes,omitempty"`
	IsRead       bool                 `json:"isRead"`
	IsStarred    bool                 `json:"isStarred"`
	MemberCount  int                  `json:"memberCount"`
	CreatedAt    time.Time            `json:"createdAt"`
	UpdatedAt    time.Time            `json:"updatedAt"`
	ArticleIDs   []string             `json:"articleIds,omitempty"`
	Articles     []Article            `json:"articles,omitempty"`
}

type Folder struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	FeedIDs   []string  `json:"feedIds"`
}

type Settings struct {
	DefaultPollIntervalSeconds int    `json:"defaultPollIntervalSeconds"`
	Theme                      string `json:"theme"`
	ArticleDensity             string `json:"articleDensity"`
	DefaultSort                string `json:"defaultSort"`
	MarkReadOnOpen             bool   `json:"markReadOnOpen"`
	NotificationsEnabled       bool   `json:"notificationsEnabled"`
	AIEnabled                  bool   `json:"aiEnabled"`
	AIBaseURL                  string `json:"aiBaseUrl"`
	AIModel                    string `json:"aiModel"`
	ReadLaterChrome            string `json:"readLaterChrome"` // tabs | brandControl
}

type ArticleQuery struct {
	FeedID           string
	FolderID         string
	UnreadOnly       bool
	StarredOnly      bool
	Search           string
	Limit            int
	Cursor           string
	DefaultSort      string
	Since            *time.Time
	ReadLaterOnly    bool
	ArchivedOnly     bool
	ExcludeArchived  bool
	ExcludeReadLater bool
}

type ArticleListResult struct {
	Articles   []Article `json:"articles"`
	NextCursor string    `json:"nextCursor"`
}

type FeedPreview struct {
	URL          string `json:"url"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	SiteURL      string `json:"siteUrl"`
	ArticleCount int    `json:"articleCount"`
}

type FeedImportResult struct {
	Added  int      `json:"added"`
	Failed int      `json:"failed"`
	Errors []string `json:"errors"`
}

type AIStatus struct {
	Running   bool   `json:"running"`
	Processed int    `json:"processed"`
	Total     int    `json:"total"`
	Pending   int    `json:"pending"`
	Failed    int    `json:"failed"`
	LastError string `json:"lastError"`
}

type AITestResult struct {
	OK      bool     `json:"ok"`
	Message string   `json:"message"`
	Models  []string `json:"models,omitempty"`
}

type AILogEntry struct {
	ID        string `json:"id"`
	TS        string `json:"ts"`
	Level     string `json:"level"`
	ArticleID string `json:"articleId,omitempty"`
	Message   string `json:"message"`
	Detail    string `json:"detail,omitempty"`
}

type AIQueueItem struct {
	ArticleID  string `json:"articleId"`
	Status     string `json:"status"`
	Attempts   int    `json:"attempts"`
	LastError  string `json:"lastError"`
	EnqueuedAt string `json:"enqueuedAt"`
}

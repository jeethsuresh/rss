package domain

import "time"

type Priority string

const (
	PriorityNone   Priority = "none"
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
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
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type Article struct {
	ID           string     `json:"id"`
	FeedID       string     `json:"feedId"`
	Title        string     `json:"title"`
	URL          string     `json:"url"`
	Author       string     `json:"author"`
	Content      string     `json:"content"`
	Summary      string     `json:"summary"`
	PublishedAt  *time.Time `json:"publishedAt"`
	UpdatedAt    *time.Time `json:"updatedAt"`
	ExternalID   string     `json:"externalId"`
	IsRead       bool       `json:"isRead"`
	IsStarred    bool       `json:"isStarred"`
	Priority     Priority   `json:"priority"`
	StoryID      string     `json:"storyId,omitempty"`
	DiscoveredAt time.Time  `json:"discoveredAt"`
	FeedTitle    string     `json:"feedTitle,omitempty"`
}

type Story struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	IsRead       bool      `json:"isRead"`
	IsStarred    bool      `json:"isStarred"`
	MemberCount  int       `json:"memberCount"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	ArticleIDs   []string  `json:"articleIds,omitempty"`
	Articles     []Article `json:"articles,omitempty"`
}

type Folder struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
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
}

type ArticleQuery struct {
	FeedID      string
	FolderID    string
	UnreadOnly  bool
	StarredOnly bool
	Search      string
	Limit       int
	Cursor      string
	DefaultSort string
	Since       *time.Time
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
	LastError string `json:"lastError"`
}

type AITestResult struct {
	OK      bool     `json:"ok"`
	Message string   `json:"message"`
	Models  []string `json:"models,omitempty"`
}

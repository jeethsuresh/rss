package domain

import "time"

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
	DiscoveredAt time.Time  `json:"discoveredAt"`
	FeedTitle    string     `json:"feedTitle,omitempty"`
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

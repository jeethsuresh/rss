package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidURL    = errors.New("invalid url")
	ErrInvalidFeed   = errors.New("invalid feed")
	ErrInvalidParams = errors.New("invalid params")
	ErrNetwork       = errors.New("network error")
	ErrParse         = errors.New("parse error")
)

type FeedRepository interface {
	List(ctx context.Context) ([]Feed, error)
	Get(ctx context.Context, id string) (*Feed, error)
	GetByURL(ctx context.Context, url string) (*Feed, error)
	GetReadLater(ctx context.Context) (*Feed, error)
	EnsureReadLater(ctx context.Context) (*Feed, error)
	Create(ctx context.Context, feed *Feed) error
	Update(ctx context.Context, feed *Feed) error
	Delete(ctx context.Context, id string) error
	RecordCrawlResult(ctx context.Context, feedID string, failed bool) error
}

type ArticleRepository interface {
	List(ctx context.Context, q ArticleQuery) (ArticleListResult, error)
	Get(ctx context.Context, id string) (*Article, error)
	ListIDsSince(ctx context.Context, since time.Time) ([]string, error)
	ListMissedIDs(ctx context.Context) ([]string, error)
	UpsertMany(ctx context.Context, articles []Article) (inserted int, err error)
	Update(ctx context.Context, article *Article) error
	SetPriority(ctx context.Context, id string, priority Priority) error
	SetCrawlResult(ctx context.Context, id string, status CrawlStatus, crawled string, errMsg string, unreliable bool) error
	SetLiveContent(ctx context.Context, id string, live string) error
	SetArchived(ctx context.Context, id string, archived bool) error
	FindByExternalKey(ctx context.Context, feedID, externalID, url, fingerprint string) (*Article, error)
	SearchCompact(ctx context.Context, query string, limit int) ([]Article, error)
	ListNeedingCrawl(ctx context.Context, limit int) ([]Article, error)
}

type StoryRepository interface {
	List(ctx context.Context) ([]Story, error)
	Get(ctx context.Context, id string) (*Story, error)
	Create(ctx context.Context, story *Story) error
	Update(ctx context.Context, story *Story) error
	SetMembers(ctx context.Context, storyID string, articleIDs []string) error
	AddMember(ctx context.Context, storyID, articleID string) error
	FindStoryForArticle(ctx context.Context, articleID string) (*Story, error)
	CascadeFlags(ctx context.Context, storyID string, isRead *bool, isStarred *bool) error
}

type FolderRepository interface {
	List(ctx context.Context) ([]Folder, error)
	Get(ctx context.Context, id string) (*Folder, error)
	Create(ctx context.Context, folder *Folder) error
	Delete(ctx context.Context, id string) error
	AssignFeed(ctx context.Context, folderID, feedID string) error
	UnassignFeed(ctx context.Context, folderID, feedID string) error
	FeedIDsInFolder(ctx context.Context, folderID string) ([]string, error)
}

type SettingsRepository interface {
	Get(ctx context.Context) (*Settings, error)
	Update(ctx context.Context, settings *Settings) error
}

type AIQueueRepository interface {
	Enqueue(ctx context.Context, articleID string) error
	ClaimNext(ctx context.Context) (articleID string, ok bool, err error)
	MarkDone(ctx context.Context, articleID string) error
	MarkFailed(ctx context.Context, articleID string, errMsg string) error
	ResetRunning(ctx context.Context) error
	Counts(ctx context.Context) (pending, running, done, failed int, err error)
	ListRecent(ctx context.Context, limit int) ([]AIQueueItem, error)
	RetryFailed(ctx context.Context) (int, error)
}

type AILogRepository interface {
	Append(ctx context.Context, entry AILogEntry) error
	List(ctx context.Context, limit int) ([]AILogEntry, error)
}

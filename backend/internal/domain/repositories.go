package domain

import (
	"context"
	"errors"
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
	Create(ctx context.Context, feed *Feed) error
	Update(ctx context.Context, feed *Feed) error
	Delete(ctx context.Context, id string) error
}

type ArticleRepository interface {
	List(ctx context.Context, q ArticleQuery) (ArticleListResult, error)
	Get(ctx context.Context, id string) (*Article, error)
	UpsertMany(ctx context.Context, articles []Article) (inserted int, err error)
	Update(ctx context.Context, article *Article) error
	FindByExternalKey(ctx context.Context, feedID, externalID, url, fingerprint string) (*Article, error)
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

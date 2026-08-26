package serverstore

import "time"

type User struct {
	ID       string    `json:"id"`
	Username string    `json:"username"`
	Created  time.Time `json:"createdAt"`
}

type AuthResult struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	User      User      `json:"user"`
}

// FeedOp is an immutable LWW CRDT event. LogicalClock is supplied by the
// originating device; ties are resolved by device ID and then operation ID.
type FeedOp struct {
	Sequence        int64  `json:"sequence,omitempty"`
	OpID            string `json:"opId"`
	FeedURL         string `json:"feedUrl"`
	Present         bool   `json:"present"`
	LogicalClock    int64  `json:"logicalClock"`
	DeviceID        string `json:"deviceId"`
	ClientCreatedAt string `json:"clientCreatedAt,omitempty"`
}

type SyncRequest struct {
	Cursor int64    `json:"cursor"`
	Ops    []FeedOp `json:"ops"`
}

type SyncResponse struct {
	Cursor  int64    `json:"cursor"`
	HasMore bool     `json:"hasMore"`
	Ops     []FeedOp `json:"ops"`
}

type ArticleStatePatch struct {
	IsRead    *bool `json:"isRead,omitempty"`
	IsStarred *bool `json:"isStarred,omitempty"`
}

type ReadLaterItem struct {
	ID               string     `json:"id"`
	URL              string     `json:"url"`
	FinalURL         string     `json:"finalUrl"`
	Title            string     `json:"title"`
	CrawledContent   string     `json:"crawledContent"`
	ReaderContent    string     `json:"readerContent"`
	CrawlStatus      string     `json:"crawlStatus"`
	CrawlError       string     `json:"crawlError"`
	FetchedAt        *time.Time `json:"fetchedAt,omitempty"`
	IsRead           bool       `json:"isRead"`
	IsStarred        bool       `json:"isStarred"`
	ArchivedAt       *time.Time `json:"archivedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	SharedDocumentID string     `json:"sharedDocumentId"`
}

type ReadLaterPatch struct {
	IsRead    *bool `json:"isRead,omitempty"`
	IsStarred *bool `json:"isStarred,omitempty"`
	Archived  *bool `json:"archived,omitempty"`
}

type FeedClaim struct {
	FeedID string
	Owner  string
}

type DocumentClaim struct {
	DocumentID string
	URL        string
	Owner      string
}

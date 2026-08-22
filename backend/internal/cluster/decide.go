package cluster

import (
	"math"
	"sort"
	"strconv"
	"time"
)

const (
	JoinThreshold      = 0.35
	StaleJoinThreshold = 0.70
	StaleAge           = 72 * time.Hour
	ArticleWindow      = 7 * 24 * time.Hour
	StoryMemberWindow  = 14 * 24 * time.Hour
)

const (
	ActionNone   = "none"
	ActionJoin   = "join"
	ActionCreate = "create"
)

type Candidate struct {
	ID      string
	StoryID string
	Vec     Vector
	At      time.Time
}

type StoryCandidate struct {
	ID        string
	MemberIDs []string
	Centroid  Vector
	Newest    time.Time
}

type Member struct {
	ID    string
	Title string
	Vec   Vector
}

type Suggestion struct {
	Action     string
	StoryID    string
	MemberIDs  []string
	NeighborID string
	Score      float64
	Threshold  float64
	Title      string
	Tokens     []string
}

func Cosine(a, b Vector) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var dot, na, nb float64
	for k, va := range a {
		na += va * va
		if vb, ok := b[k]; ok {
			dot += va * vb
		}
	}
	for _, vb := range b {
		nb += vb * vb
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func Mean(vecs []Vector) Vector {
	out := Vector{}
	if len(vecs) == 0 {
		return out
	}
	for _, v := range vecs {
		for k, val := range v {
			out[k] += val
		}
	}
	n := float64(len(vecs))
	for k := range out {
		out[k] /= n
	}
	return out
}

func FormatTitle(n int, articleTitle string) string {
	if articleTitle == "" {
		articleTitle = "untitled"
	}
	return "(" + strconv.Itoa(n) + ") " + articleTitle
}

func CentroidTitle(members []Member) (id, title string) {
	if len(members) == 0 {
		return "", ""
	}
	vecs := make([]Vector, 0, len(members))
	for _, m := range members {
		vecs = append(vecs, m.Vec)
	}
	cent := Mean(vecs)
	best := 0
	bestScore := math.Inf(-1)
	for i, m := range members {
		s := Cosine(m.Vec, cent)
		if s > bestScore {
			bestScore = s
			best = i
		}
	}
	return members[best].ID, members[best].Title
}

func OverlapTokens(article, rest Vector) []string {
	out := make([]string, 0)
	for k := range article {
		if _, ok := rest[k]; ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func Decide(article Candidate, articles []Candidate, stories []StoryCandidate, now time.Time, excludeStory map[string]bool) Suggestion {
	return DecideExcluding(article, articles, stories, now, excludeStory, nil)
}

func DecideExcluding(article Candidate, articles []Candidate, stories []StoryCandidate, now time.Time, excludeStory, excludeArticle map[string]bool) Suggestion {
	type hit struct {
		score      float64
		storyID    string
		neighborID string
		newest     time.Time
		isStory    bool
	}
	best := hit{score: -1}
	consider := func(h hit) {
		if h.score > best.score {
			best = h
		}
	}

	for _, a := range articles {
		if a.ID == "" || a.ID == article.ID || excludeArticle[a.ID] {
			continue
		}
		if excludeStory[a.StoryID] {
			continue
		}
		consider(hit{
			score:      Cosine(article.Vec, a.Vec),
			storyID:    a.StoryID,
			neighborID: a.ID,
			newest:     a.At,
			isStory:    a.StoryID != "",
		})
	}
	for _, s := range stories {
		if s.ID == "" || excludeStory[s.ID] {
			continue
		}
		consider(hit{
			score:   Cosine(article.Vec, s.Centroid),
			storyID: s.ID,
			newest:  s.Newest,
			isStory: true,
		})
	}

	out := Suggestion{Action: ActionNone, Score: best.score, Threshold: JoinThreshold, Tokens: tokenList(article.Vec)}
	if best.score < 0 {
		out.Score = 0
		return out
	}
	out.NeighborID = best.neighborID
	if best.isStory && best.storyID != "" {
		out.Threshold = JoinThreshold
		if now.Sub(best.newest) > StaleAge {
			out.Threshold = StaleJoinThreshold
		}
		if best.score >= out.Threshold {
			out.Action = ActionJoin
			out.StoryID = best.storyID
			out.MemberIDs = []string{article.ID}
		}
		return out
	}
	if best.neighborID != "" && best.score >= JoinThreshold {
		out.Action = ActionCreate
		out.MemberIDs = []string{article.ID, best.neighborID}
	}
	return out
}

func tokenList(v Vector) []string {
	out := make([]string, 0, len(v))
	for k := range v {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

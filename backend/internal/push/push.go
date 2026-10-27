// Package push owns browser subscriptions, activity rounds, and an encrypted
// Web Push outbox. Notification scheduling runs entirely on the server.
package push

import (
	"context"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	webpush "github.com/SherClockHolmes/webpush-go"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const OfficeQuiet = 10 * time.Minute
const OfficeContinuous = 30 * time.Minute
const NewsInterval = 24 * time.Hour

var ErrInvalid = errors.New("invalid push subscription")
var ErrLimit = errors.New("too many push subscriptions")

type Subscription = webpush.Subscription
type Device struct {
	Owner        string       `json:"owner"`
	Session      string       `json:"session"`
	Binding      string       `json:"binding"`
	Expires      time.Time    `json:"expires"`
	Subscription Subscription `json:"subscription"`
	Topics       []string     `json:"topics"`
}
type Notification struct {
	Account string `json:"account"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	URL     string `json:"url"`
	Tag     string `json:"tag"`
}
type round struct {
	Owner  string       `json:"owner"`
	Kind   string       `json:"kind"`
	Notice Notification `json:"notice"`
	First  time.Time    `json:"first"`
	Last   time.Time    `json:"last"`
	Due    time.Time    `json:"due"`
	Sent   bool         `json:"sent"`
}
type job struct {
	Owner    string            `json:"owner"`
	Kind     string            `json:"kind"`
	Notice   Notification      `json:"notice"`
	Created  time.Time         `json:"created"`
	Due      time.Time         `json:"due"`
	Targets  map[string]string `json:"targets"`
	Done     map[string]bool   `json:"done"`
	Attempts int               `json:"attempts"`
}
type state struct {
	Private  string               `json:"private"`
	Public   string               `json:"public"`
	Devices  map[string]Device    `json:"devices"`
	Rounds   map[string]round     `json:"rounds"`
	Jobs     map[string]job       `json:"jobs"`
	LastNews map[string]time.Time `json:"lastNews"`
	Started  map[string]time.Time `json:"started"`
}
type Service struct {
	mu        sync.Mutex
	path      string
	state     state
	dirty     bool
	Client    webpush.HTTPClient
	Validate  func(context.Context, Device) (bool, error)
	NewsReady func(context.Context, string, time.Time) (bool, error)
	Now       func() time.Time
}

func Open(path string) (*Service, error) {
	s := &Service{path: path, Now: time.Now, Client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	data, e := os.ReadFile(path)
	if e == nil {
		if e = json.Unmarshal(data, &s.state); e != nil {
			return nil, e
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if s.state.Private == "" || s.state.Public == "" {
		s.state.Private, s.state.Public, e = webpush.GenerateVAPIDKeys()
		if e != nil {
			return nil, e
		}
	}
	if s.state.Devices == nil {
		s.state.Devices = map[string]Device{}
	}
	if s.state.Rounds == nil {
		s.state.Rounds = map[string]round{}
	}
	if s.state.Jobs == nil {
		s.state.Jobs = map[string]job{}
	}
	if s.state.LastNews == nil {
		s.state.LastNews = map[string]time.Time{}
	}
	if s.state.Started == nil {
		s.state.Started = map[string]time.Time{}
	}
	if e = s.save(); e != nil {
		return nil, e
	}
	return s, nil
}
func (s *Service) save() error {
	if e := os.MkdirAll(filepath.Dir(s.path), 0700); e != nil {
		return e
	}
	data, e := json.Marshal(s.state)
	if e != nil {
		return e
	}
	file, e := os.CreateTemp(filepath.Dir(s.path), ".push-*")
	if e != nil {
		return e
	}
	defer os.Remove(file.Name())
	if e = file.Chmod(0600); e == nil {
		_, e = file.Write(data)
	}
	if e == nil {
		e = file.Sync()
	}
	if ce := file.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(file.Name(), s.path)
	}
	if e == nil {
		s.dirty = false
	}
	return e
}
func (s *Service) PublicKey() string { s.mu.Lock(); defer s.mu.Unlock(); return s.state.Public }
func id(value string) string         { sum := sha256.Sum256([]byte(value)); return hex.EncodeToString(sum[:]) }
func SessionKey(value string) string { return id(value) }
func valid(sub Subscription) bool {
	u, e := url.Parse(sub.Endpoint)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") || len(sub.Endpoint) > 4096 {
		return false
	}
	host := strings.ToLower(u.Hostname())
	allowed := host == "fcm.googleapis.com" || host == "updates.push.services.mozilla.com" || host == "push.services.mozilla.com" || host == "web.push.apple.com" || strings.HasSuffix(host, ".push.apple.com") || strings.HasSuffix(host, ".notify.windows.com")
	if !allowed {
		return false
	}
	public, e := base64.RawURLEncoding.DecodeString(sub.Keys.P256dh)
	if e != nil {
		return false
	}
	x, y := elliptic.Unmarshal(elliptic.P256(), public)
	auth, e := base64.RawURLEncoding.DecodeString(sub.Keys.Auth)
	return e == nil && len(auth) == 16 && x != nil && y != nil
}
func (s *Service) Subscribe(d Device) error {
	if d.Owner == "" || d.Binding == "" || !d.Expires.After(s.Now()) || !valid(d.Subscription) {
		return ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, device := range s.state.Devices {
		if !device.Expires.After(s.Now()) {
			s.removeDevice(key)
		}
	}
	key := id(d.Subscription.Endpoint)
	old, existed := s.state.Devices[key]
	if existed && old.Owner != d.Owner {
		s.removeDevice(key)
	}
	count := 0
	for _, device := range s.state.Devices {
		if device.Owner == d.Owner && device.Expires.After(s.Now()) {
			count++
		}
	}
	if !existed && (count >= 20 || len(s.state.Devices) >= 10000) {
		return ErrLimit
	}
	if existed && old.Owner == d.Owner {
		d.Topics = old.Topics
	}
	s.state.Devices[key] = d
	if s.state.Started[d.Owner].IsZero() {
		s.state.Started[d.Owner] = s.Now()
	}
	if e := s.save(); e != nil {
		if existed {
			s.state.Devices[key] = old
		} else {
			s.removeDevice(key)
		}
		return e
	}
	return nil
}

// removeDevice also cancels pending delivery to that endpoint. Re-enabling a
// browser must not resurrect a previously disabled account's queued notices.
func (s *Service) removeDevice(key string) {
	d, ok := s.state.Devices[key]
	if !ok {
		return
	}
	delete(s.state.Devices, key)
	for k, j := range s.state.Jobs {
		delete(j.Targets, key)
		delete(j.Done, key)
		if len(j.Targets) == 0 {
			delete(s.state.Jobs, k)
		} else {
			s.state.Jobs[k] = j
		}
	}
	for _, other := range s.state.Devices {
		if other.Owner == d.Owner && other.Expires.After(s.Now()) {
			return
		}
	}
	for k, r := range s.state.Rounds {
		if r.Owner == d.Owner {
			delete(s.state.Rounds, k)
		}
	}
}
func (s *Service) Unsubscribe(owner, endpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := id(endpoint)
	if d, ok := s.state.Devices[key]; ok && d.Owner == owner {
		s.removeDevice(key)
	}
	return s.save()
}
func (s *Service) Revoke(binding string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, d := range s.state.Devices {
		if d.Binding == binding || d.Session == SessionKey(binding) {
			s.removeDevice(key)
		}
	}
	return s.save()
}
func category(topic string) string {
	if topic == "news" {
		return "news"
	}
	return "office"
}
func officeDue(r round) time.Time {
	return minTime(r.Last.Add(OfficeQuiet), r.First.Add(OfficeContinuous))
}
func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// Notify records activity once per recipient, independent of device count. Own
// edits extend an existing collaboration round but cannot create an own-edit alert.
func (s *Service) Notify(topic, author string, owners map[string]bool, notice Notification) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Now()
	kind := category(topic)
	eligible := map[string]bool{}
	for _, d := range s.state.Devices {
		if !d.Expires.After(now) || (owners != nil && !owners[d.Owner]) {
			continue
		}
		allowed := kind == "news"
		for _, t := range d.Topics {
			if t == topic || t == "*" {
				allowed = true
			}
		}
		if allowed {
			eligible[d.Owner] = true
		}
	}
	changed, first := false, false
	for owner := range eligible {
		key := owner + ":" + kind
		r, exists := s.state.Rounds[key]
		if kind == "office" && exists && now.Sub(r.Last) >= OfficeQuiet {
			if !r.Sent {
				s.seal(key, r, now)
			}
			delete(s.state.Rounds, key)
			exists = false
		}
		if owner == author {
			if exists {
				r.Last = now
				if !r.Sent {
					r.Due = officeDue(r)
				}
				s.state.Rounds[key] = r
				changed = true
			}
			continue
		}
		if !exists {
			if len(s.state.Rounds) >= 10000 {
				return ErrLimit
			}
			r = round{Owner: owner, Kind: kind, First: now}
			first = true
		}
		r.Last = now
		r.Notice = notice
		if kind == "office" {
			if !r.Sent {
				r.Due = officeDue(r)
			}
		} else {
			r.Due = now
			if last := s.state.LastNews[owner]; last.Add(NewsInterval).After(r.Due) {
				r.Due = last.Add(NewsInterval)
			}
		}
		s.state.Rounds[key] = r
		changed = true
	}
	if changed {
		s.dirty = true
	}
	if first {
		return s.save()
	}
	return nil
}
func (s *Service) seal(key string, r round, now time.Time) {
	if len(s.state.Jobs) >= 10000 {
		return
	}
	targets := map[string]string{}
	for k, d := range s.state.Devices {
		if d.Owner == r.Owner && d.Expires.After(now) {
			targets[k] = d.Session
		}
	}
	if len(targets) > 0 {
		s.state.Jobs[key+":"+r.First.UTC().Format(time.RFC3339Nano)] = job{Owner: r.Owner, Kind: r.Kind, Notice: r.Notice, Created: now, Due: now, Targets: targets, Done: map[string]bool{}}
	}
	s.dirty = true
}
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			if s.dirty {
				_ = s.save()
			}
			s.mu.Unlock()
			return
		case <-ticker.C:
			s.Dispatch(ctx, s.Now())
		}
	}
}
func (s *Service) Dispatch(ctx context.Context, now time.Time) {
	s.mu.Lock()
	for key, r := range s.state.Rounds {
		if r.Kind == "office" {
			if !r.Sent && !r.Due.After(now) {
				s.seal(key, r, now)
				r.Sent = true
				s.state.Rounds[key] = r
				s.dirty = true
			}
			if now.Sub(r.Last) >= OfficeQuiet {
				delete(s.state.Rounds, key)
				s.dirty = true
			}
			continue
		}
		if r.Due.After(now) {
			continue
		}
		waiting := false
		for _, j := range s.state.Jobs {
			if j.Owner == r.Owner && j.Kind == "news" {
				waiting = true
				break
			}
		}
		if waiting {
			continue
		}
		since := s.state.LastNews[r.Owner]
		if since.IsZero() {
			since = s.state.Started[r.Owner]
		}
		if !s.state.LastNews[r.Owner].IsZero() && now.Before(since.Add(NewsInterval)) {
			r.Due = since.Add(NewsInterval)
			s.state.Rounds[key] = r
			continue
		}
		ready := false
		if s.NewsReady != nil {
			check, cancel := context.WithTimeout(ctx, 5*time.Second)
			ready, _ = s.NewsReady(check, r.Owner, since)
			cancel()
		}
		if !ready {
			r.Due = now.Add(time.Minute)
			s.state.Rounds[key] = r
			s.dirty = true
			continue
		}
		s.seal(key, r, now)
		delete(s.state.Rounds, key)
		s.dirty = true
	}
	if s.dirty {
		if s.save() != nil {
			s.mu.Unlock()
			return
		}
	}
	due := map[string]job{}
	for key, j := range s.state.Jobs {
		if !j.Due.After(now) && len(due) < 8 {
			due[key] = j
		}
	}
	s.mu.Unlock()
	for key, j := range due {
		s.deliver(ctx, key, j, now)
	}
}
func (s *Service) deliver(ctx context.Context, key string, j job, now time.Time) {
	for target, session := range j.Targets {
		if j.Done[target] {
			continue
		}
		s.mu.Lock()
		d, ok := s.state.Devices[target]
		private, public := s.state.Private, s.state.Public
		s.mu.Unlock()
		if !ok || d.Owner != j.Owner || d.Session != session || !d.Expires.After(now) {
			j.Done[target] = true
			continue
		}
		// A delayed daily batch must not race a newer accepted batch.
		s.mu.Lock()
		last := s.state.LastNews[j.Owner]
		s.mu.Unlock()
		if j.Kind == "news" && last.After(j.Created) && now.Sub(last) > time.Hour {
			j.Done[target] = true
			continue
		}
		timeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		valid := false
		var e error
		if s.Validate != nil {
			valid, e = s.Validate(timeout, d)
		} else {
			e = errors.New("push authorization is not ready")
		}
		code := 0
		if valid && e == nil {
			notice := j.Notice
			notice.Account = d.Owner
			body, _ := json.Marshal(notice)
			response, err := webpush.SendNotificationWithContext(timeout, body, &d.Subscription, &webpush.Options{HTTPClient: s.Client, Subscriber: "https://gravitas.cool", VAPIDPublicKey: public, VAPIDPrivateKey: private, TTL: 3600, Urgency: webpush.UrgencyLow, Topic: id(j.Kind)[:24]})
			if err == nil {
				code = response.StatusCode
				_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
				_ = response.Body.Close()
			}
		}
		cancel()
		s.mu.Lock()
		current, exists := s.state.Devices[target]
		if exists && current.Session == d.Session && current.Owner == d.Owner {
			switch {
			case !valid && e == nil, code == 404, code == 410:
				delete(s.state.Devices, target)
				j.Done[target] = true
			case code >= 200 && code < 300:
				j.Done[target] = true
				if j.Kind == "news" && s.state.LastNews[j.Owner].Before(j.Created) {
					s.state.LastNews[j.Owner] = s.Now()
				}
			}
		} else {
			j.Done[target] = true
		}
		s.mu.Unlock()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	complete := true
	for target := range j.Targets {
		if !j.Done[target] {
			complete = false
		}
	}
	if complete || now.Sub(j.Created) > 24*time.Hour {
		delete(s.state.Jobs, key)
	} else {
		j.Attempts++
		j.Due = now.Add(min(time.Hour, 30*time.Second*time.Duration(1<<min(j.Attempts, 7))))
		s.state.Jobs[key] = j
	}
	s.dirty = true
	_ = s.save()
}

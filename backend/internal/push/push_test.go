package push

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	webpush "github.com/SherClockHolmes/webpush-go"
	"golang.org/x/crypto/hkdf"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) Do(r *http.Request) (*http.Response, error) { return f(r) }
func device(t *testing.T, owner, endpoint string) (Device, *ecdh.PrivateKey, []byte) {
	t.Helper()
	private, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	return Device{Owner: owner, Session: SessionKey(owner), Binding: owner, Expires: time.Now().Add(14 * 24 * time.Hour), Topics: []string{"document"}, Subscription: Subscription{Endpoint: "https://fcm.googleapis.com/" + endpoint, Keys: webpush.Keys{Auth: base64.RawURLEncoding.EncodeToString(auth), P256dh: base64.RawURLEncoding.EncodeToString(private.PublicKey().Bytes())}}}, private, auth
}
func open(t *testing.T) *Service {
	t.Helper()
	s, e := Open(filepath.Join(t.TempDir(), "push.json"))
	if e != nil {
		t.Fatal(e)
	}
	s.Validate = func(context.Context, Device) (bool, error) { return true, nil }
	return s
}
func decode(t *testing.T, data []byte, private *ecdh.PrivateKey, auth []byte) Notification {
	t.Helper()
	if len(data) < 86 {
		t.Fatal("missing encrypted Web Push header")
	}
	public, err := ecdh.P256().NewPublicKey(data[21:86])
	if err != nil {
		t.Fatal(err)
	}
	shared, err := private.ECDH(public)
	if err != nil {
		t.Fatal(err)
	}
	info := append(append([]byte("WebPush: info\x00"), private.PublicKey().Bytes()...), public.Bytes()...)
	derive := func(secret, salt, info []byte, n int) []byte {
		out := make([]byte, n)
		if _, e := io.ReadFull(hkdf.New(sha256.New, secret, salt, info), out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	ikm := derive(shared, auth, info, 32)
	key := derive(ikm, data[:16], []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := derive(ikm, data[:16], []byte("Content-Encoding: nonce\x00"), 12)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, data[86:], nil)
	if err != nil {
		t.Fatal(err)
	}
	for len(plain) > 0 && plain[len(plain)-1] == 0 {
		plain = plain[:len(plain)-1]
	}
	if len(plain) == 0 || plain[len(plain)-1] != 2 {
		t.Fatal("invalid Web Push padding")
	}
	var notice Notification
	if err = json.Unmarshal(plain[:len(plain)-1], &notice); err != nil {
		t.Fatal(err)
	}
	return notice
}

func clock(t *testing.T) (*Service, *time.Time, *int) {
	t.Helper()
	s := open(t)
	now := time.Now().UTC()
	s.Now = func() time.Time { return now }
	calls := 0
	s.Client = transport(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	return s, &now, &calls
}
func subscribe(t *testing.T, s *Service, owner, endpoint string) {
	t.Helper()
	d, _, _ := device(t, owner, endpoint)
	d.Topics = []string{"*"}
	if e := s.Subscribe(d); e != nil {
		t.Fatal(e)
	}
}
func activity(t *testing.T, s *Service, topic, author string) {
	t.Helper()
	if e := s.Notify(topic, author, nil, Notification{Title: "Updates", URL: "/"}); e != nil {
		t.Fatal(e)
	}
}
func TestEncryptedDeliveryPersistsAndCoalesces(t *testing.T) {
	s, now, _ := clock(t)
	d, private, auth := device(t, "alice", "one")
	if e := s.Subscribe(d); e != nil {
		t.Fatal(e)
	}
	notice := Notification{Title: "Collaborator updates", Body: "A round of changes is ready.", URL: "/#document=document", Tag: "office-collaboration"}
	for i := 0; i < 50; i++ {
		if e := s.Notify("document", "bob", nil, notice); e != nil {
			t.Fatal(e)
		}
	}
	if len(s.state.Rounds) != 1 || len(s.state.Jobs) != 0 {
		t.Fatal("typing burst did not become one pending round")
	}
	reopened, e := Open(s.path)
	if e != nil {
		t.Fatal(e)
	}
	if reopened.PublicKey() != s.PublicKey() || len(reopened.state.Rounds) != 1 {
		t.Fatal("keys/subscriptions/round did not survive restart")
	}
	reopened.Now = s.Now
	reopened.Validate = s.Validate
	calls := 0
	reopened.Client = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		if !strings.HasPrefix(r.Header.Get("Authorization"), "vapid ") || r.Header.Get("Content-Encoding") != "aes128gcm" {
			t.Fatal("missing Web Push authentication/encryption")
		}
		body, _ := io.ReadAll(r.Body)
		got := decode(t, body, private, auth)
		if got.Account != "alice" || got.Title != notice.Title || got.URL != notice.URL {
			t.Fatalf("wrong encrypted notification: %+v", got)
		}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	*now = now.Add(OfficeQuiet - time.Second)
	reopened.Dispatch(context.Background(), *now)
	if calls != 0 {
		t.Fatal("sent before quiet period")
	}
	*now = now.Add(time.Second)
	reopened.Dispatch(context.Background(), *now)
	reopened.Dispatch(context.Background(), *now)
	if calls != 1 || len(reopened.state.Jobs) != 0 {
		t.Fatal("round was not delivered once")
	}
	info, _ := os.Stat(s.path)
	if info.Mode().Perm()&0077 != 0 {
		t.Fatal("push secrets are not private")
	}
}
func TestOfficeQuietPeriodResetsAcrossDocumentsAndDevices(t *testing.T) {
	s, now, calls := clock(t)
	subscribe(t, s, "alice", "one")
	subscribe(t, s, "alice", "two")
	activity(t, s, "document-a", "bob")
	*now = now.Add(9 * time.Minute)
	activity(t, s, "document-b", "carol")
	*now = now.Add(time.Minute)
	s.Dispatch(context.Background(), *now)
	if *calls != 0 {
		t.Fatal("sent while collaborators were still active")
	}
	*now = now.Add(9 * time.Minute)
	s.Dispatch(context.Background(), *now)
	if *calls != 2 {
		t.Fatalf("want one round on each of two devices, got %d", *calls)
	}
	s.Dispatch(context.Background(), *now)
	if *calls != 2 {
		t.Fatal("duplicate round")
	}
	activity(t, s, "document-c", "bob")
	*now = now.Add(OfficeQuiet)
	s.Dispatch(context.Background(), *now)
	if *calls != 4 {
		t.Fatal("new round after quiet gap was lost")
	}
}
func TestOfficeContinuousRoundSendsOnceAtThirtyMinutes(t *testing.T) {
	s, now, calls := clock(t)
	subscribe(t, s, "alice", "one")
	start := *now
	activity(t, s, "a", "bob")
	for minute := 1; minute <= 60; minute++ {
		*now = start.Add(time.Duration(minute) * time.Minute)
		activity(t, s, "b", "bob")
		s.Dispatch(context.Background(), *now)
		want := 0
		if minute >= 30 {
			want = 1
		}
		if *calls != want {
			t.Fatalf("minute %d: want %d deliveries, got %d", minute, want, *calls)
		}
	}
	*now = now.Add(OfficeQuiet)
	s.Dispatch(context.Background(), *now)
	if *calls != 1 {
		t.Fatal("quiet close duplicated continuous-round alert")
	}
	activity(t, s, "a", "bob")
	*now = now.Add(OfficeQuiet)
	s.Dispatch(context.Background(), *now)
	if *calls != 2 {
		t.Fatal("next round did not notify")
	}
}
func TestOwnEditsExtendButDoNotCreateOfficeRound(t *testing.T) {
	s, now, calls := clock(t)
	subscribe(t, s, "alice", "one")
	activity(t, s, "a", "alice")
	*now = now.Add(OfficeQuiet)
	s.Dispatch(context.Background(), *now)
	if *calls != 0 || len(s.state.Rounds) != 0 {
		t.Fatal("own edit created an alert")
	}
	activity(t, s, "a", "bob")
	*now = now.Add(9 * time.Minute)
	activity(t, s, "a", "alice")
	*now = now.Add(time.Minute)
	s.Dispatch(context.Background(), *now)
	if *calls != 0 {
		t.Fatal("own activity did not extend quiet period")
	}
	*now = now.Add(9 * time.Minute)
	s.Dispatch(context.Background(), *now)
	if *calls != 1 {
		t.Fatal("collaborator round was lost")
	}
}
func TestNewsWaitsForAllFeedsAndDailyLimitSurvivesRestart(t *testing.T) {
	s, now, calls := clock(t)
	subscribe(t, s, "alice", "one")
	started := *now
	ready := false
	var checked time.Time
	s.NewsReady = func(_ context.Context, owner string, since time.Time) (bool, error) {
		if owner != "alice" {
			t.Fatal("wrong owner")
		}
		checked = since
		return ready, nil
	}
	activity(t, s, "news", "")
	s.Dispatch(context.Background(), *now)
	if *calls != 0 || !checked.Equal(started) {
		t.Fatal("sent before full refresh or used wrong initial boundary")
	}
	ready = true
	*now = now.Add(time.Minute)
	s.Dispatch(context.Background(), *now)
	if *calls != 1 {
		t.Fatal("completed refresh did not notify")
	}
	last := *now
	activity(t, s, "news", "")
	*now = now.Add(23 * time.Hour)
	s.Dispatch(context.Background(), *now)
	if *calls != 1 {
		t.Fatal("exceeded daily limit")
	}
	reopened, e := Open(s.path)
	if e != nil {
		t.Fatal(e)
	}
	reopened.Now = s.Now
	reopened.Client = s.Client
	reopened.Validate = s.Validate
	reopened.NewsReady = s.NewsReady
	*now = last.Add(24*time.Hour - time.Second)
	reopened.Dispatch(context.Background(), *now)
	if *calls != 1 {
		t.Fatal("restart reset daily limit")
	}
	*now = last.Add(24 * time.Hour)
	reopened.Dispatch(context.Background(), *now)
	if *calls != 2 || !checked.Equal(last) {
		t.Fatal("next daily round did not require every feed refreshed since previous alert")
	}
	reopened.Dispatch(context.Background(), *now)
	if *calls != 2 {
		t.Fatal("duplicate daily delivery")
	}
}
func TestSubscriptionsRejectSSRFAndKeepOwnersSeparate(t *testing.T) {
	s, now, calls := clock(t)
	a, _, _ := device(t, "alice", "one")
	b, _, _ := device(t, "bob", "two")
	for _, endpoint := range []string{"http://fcm.googleapis.com/x", "https://localhost/x", "https://127.0.0.1/x", "https://fcm.googleapis.com.attacker.test/x", "https://user@fcm.googleapis.com/x", "https://fcm.googleapis.com:8443/x"} {
		bad := a
		bad.Subscription.Endpoint = endpoint
		if s.Subscribe(bad) == nil {
			t.Fatal("accepted unsafe endpoint", endpoint)
		}
	}
	_ = s.Subscribe(a)
	_ = s.Subscribe(b)
	activity(t, s, "document", "alice")
	if len(s.state.Rounds) != 1 {
		t.Fatal("author received their own edit")
	}
	_ = s.Unsubscribe("alice", b.Subscription.Endpoint)
	if len(s.state.Devices) != 2 {
		t.Fatal("another account removed a subscription")
	}
	_ = s.Revoke("bob")
	*now = now.Add(OfficeQuiet)
	s.Dispatch(context.Background(), *now)
	if *calls != 0 || len(s.state.Devices) != 1 {
		t.Fatal("logout did not suppress queued event")
	}
	activity(t, s, "document", "bob")
	*now = now.Add(OfficeQuiet)
	s.Dispatch(context.Background(), *now)
	// Assigning the same endpoint to another account cannot deliver the previous owner's retry.
	s.Client = transport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	activity(t, s, "document", "bob")
	*now = now.Add(OfficeQuiet)
	s.Dispatch(context.Background(), *now)
	a.Owner = "carol"
	a.Binding = "carol"
	a.Session = SessionKey("carol")
	_ = s.Subscribe(a)
	s.Client = transport(func(*http.Request) (*http.Response, error) {
		t.Fatal("old account job reached reassigned endpoint")
		return nil, nil
	})
	*now = now.Add(time.Hour)
	s.Dispatch(context.Background(), *now)
}
func TestRetryGoneAndRevocation(t *testing.T) {
	for _, code := range []int{503, 410} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			s, now, _ := clock(t)
			subscribe(t, s, "alice", "one")
			activity(t, s, "a", "bob")
			s.Client = transport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(""))}, nil
			})
			*now = now.Add(OfficeQuiet)
			s.Dispatch(context.Background(), *now)
			if code == 503 && len(s.state.Jobs) != 1 {
				t.Fatal("temporary failure lost outbox")
			}
			if code == 410 && len(s.state.Devices) != 0 {
				t.Fatal("gone endpoint retained")
			}
		})
	}
	s, now, _ := clock(t)
	subscribe(t, s, "alice", "one")
	activity(t, s, "a", "bob")
	s.Validate = func(context.Context, Device) (bool, error) { return false, nil }
	s.Client = transport(func(*http.Request) (*http.Response, error) { t.Fatal("revoked session sent"); return nil, nil })
	*now = now.Add(OfficeQuiet)
	s.Dispatch(context.Background(), *now)
	if len(s.state.Devices) != 0 {
		t.Fatal("revoked grant retained")
	}
}

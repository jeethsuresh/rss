package gravitas

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPushGrantIsSeparateFromAppLoginAndHonorsRevocation(t *testing.T) {
	c, auth, cookie := authorizedFixture(t)
	bound, err := c.DecodeAppSession(cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	grant, expires, err := c.SealPushAuthorization(bound)
	if err != nil {
		t.Fatal(err)
	}
	if expires.Unix() != auth.CheckExpires {
		t.Fatal("background grant does not follow the session family lifetime")
	}
	if _, err = c.DecodeAppSession(grant); !errors.Is(err, ErrLoginRequired) {
		t.Fatal("push-only grant authenticated an app session")
	}
	got, err := c.AuthenticatePushAuthorization(context.Background(), grant)
	if err != nil || got.Subject != auth.Subject {
		t.Fatalf("valid grant rejected: %v", err)
	}
	if _, err = c.AuthenticatePushAuthorization(context.Background(), cookie.Value); !errors.Is(err, ErrLoginRequired) {
		t.Fatal("app cookie accepted as a push grant")
	}
	c.HTTP.Transport = sessionRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 401, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	if _, err = c.AuthenticatePushAuthorization(context.Background(), grant); !errors.Is(err, ErrLoginRequired) {
		t.Fatal("central revocation did not stop push")
	}
	bound.ExpiresAt = time.Now().Add(-time.Second)
	if _, _, err = c.SealPushAuthorization(bound); !errors.Is(err, ErrLoginRequired) {
		t.Fatal("expired app session minted a push grant")
	}
}

package gravitas

import (
	"context"
	"time"
)

// SealPushAuthorization grants only background-notification status checks. It
// deliberately contains no local API token and cannot authenticate an app cookie.
func (c *Client) SealPushAuthorization(bound AppSession) (string, time.Time, error) {
	a := bound.Authorization
	if !bound.ExpiresAt.After(time.Now()) || !validBinding(a) || !c.acceptsIssuer(a.Issuer) {
		return "", time.Time{}, ErrLoginRequired
	}
	expires := time.Unix(a.CheckExpires, 0)
	value, err := c.seal("push-notifications", session{Identity: a.Identity, Expires: expires.Unix(), CheckToken: a.CheckToken, SessionID: a.SessionID, CheckExpires: a.CheckExpires})
	return value, expires, err
}
func (c *Client) AuthenticatePushAuthorization(ctx context.Context, value string) (Authorization, error) {
	var saved session
	if err := c.open("push-notifications", value, &saved); err != nil {
		return Authorization{}, ErrLoginRequired
	}
	a := Authorization{Identity: saved.Identity, CheckToken: saved.CheckToken, SessionID: saved.SessionID, CheckExpires: saved.CheckExpires}
	if saved.Expires != saved.CheckExpires || !validBinding(a) || !c.acceptsIssuer(a.Issuer) {
		return Authorization{}, ErrLoginRequired
	}
	return a, c.Revalidate(ctx, AppSession{Authorization: a, ExpiresAt: time.Unix(saved.Expires, 0)})
}

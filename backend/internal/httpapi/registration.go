package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
)

type registrationCredentials struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	InviteKey string `json:"inviteKey"`
}

func (a *API) registrationEnabled() bool {
	return a.cfg.Federation == nil && a.cfg.RegistrationEnabled && a.cfg.RegistrationKey != ""
}

func (a *API) authorizeInvite(w http.ResponseWriter, provided string) bool {
	expectedHash := sha256.Sum256([]byte(a.cfg.RegistrationKey))
	providedHash := sha256.Sum256([]byte(provided))
	if a.cfg.RegistrationKey == "" || subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) != 1 {
		writeError(w, http.StatusForbidden, "INVALID_INVITE_KEY", errors.New("a valid invite key is required to create an account"))
		return false
	}
	return true
}

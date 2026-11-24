package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/atlas/atlas/backend/internal/auth"
)

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &input); err != nil || strings.TrimSpace(input.Username) == "" || input.Password == "" {
		fail(w, 400, "validation_failed", "username and password are required")
		return
	}
	session, err := a.auth.Login(r.Context(), input.Username, input.Password)
	if err != nil {
		fail(w, 401, "unauthorized", "invalid username or password")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookie, Value: session.Token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Expires: session.ExpiresAt, MaxAge: int(time.Until(session.ExpiresAt).Seconds())})
	write(w, 200, map[string]any{"user": session.User, "csrfToken": session.CSRFToken, "expiresAt": session.ExpiresAt})
}
func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.SessionCookie); err == nil {
		_ = a.auth.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	write(w, 200, map[string]string{"status": "logged_out"})
}
func (a *API) me(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(auth.SessionCookie)
	if err != nil {
		fail(w, 401, "unauthorized", "login required")
		return
	}
	session, err := a.auth.Session(r.Context(), c.Value)
	if err != nil {
		fail(w, 401, "unauthorized", "invalid or expired session")
		return
	}
	write(w, 200, map[string]any{"user": session.User, "csrfToken": session.CSRFToken, "expiresAt": session.ExpiresAt})
}
func (a *API) csrf(w http.ResponseWriter, r *http.Request) { a.me(w, r) }

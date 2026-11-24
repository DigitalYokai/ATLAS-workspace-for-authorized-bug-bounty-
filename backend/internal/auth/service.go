package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const SessionCookie = "atlas_session"

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}
type Session struct {
	Token     string
	CSRFToken string
	ExpiresAt time.Time
	User      User
}
type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) Bootstrap(ctx context.Context, username, password string) error {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 12 {
		return fmt.Errorf("ATLAS_ADMIN_USERNAME and an at least 12-character ATLAS_ADMIN_PASSWORD are required for first startup")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, "INSERT INTO users(id,username,password_hash,created_at,updated_at) VALUES(?,?,?,?,?)", uuid.NewString(), username, string(hash), now, now)
	return err
}

func BootstrapFromEnvironment(ctx context.Context, db *sql.DB) error {
	return NewService(db).Bootstrap(ctx, os.Getenv("ATLAS_ADMIN_USERNAME"), os.Getenv("ATLAS_ADMIN_PASSWORD"))
}
func (s *Service) Required(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count)
	return count > 0, err
}
func token() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func hashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *Service) Login(ctx context.Context, username, password string) (Session, error) {
	var id, stored string
	err := s.db.QueryRowContext(ctx, "SELECT id,password_hash FROM users WHERE username=?", strings.TrimSpace(username)).Scan(&id, &stored)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) != nil {
		return Session{}, fmt.Errorf("invalid username or password")
	}
	t, err := token()
	if err != nil {
		return Session{}, err
	}
	csrf, err := token()
	if err != nil {
		return Session{}, err
	}
	now := time.Now().UTC()
	expires := now.Add(24 * time.Hour)
	_, err = s.db.ExecContext(ctx, "INSERT INTO sessions(id,user_id,token_hash,csrf_token,expires_at,created_at,last_seen_at) VALUES(?,?,?,?,?,?,?)", uuid.NewString(), id, hashToken(t), csrf, expires.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return Session{}, err
	}
	return Session{Token: t, CSRFToken: csrf, ExpiresAt: expires, User: User{ID: id, Username: strings.TrimSpace(username)}}, nil
}
func (s *Service) Session(ctx context.Context, value string) (Session, error) {
	var session Session
	var expires string
	err := s.db.QueryRowContext(ctx, "SELECT u.id,u.username,s.csrf_token,s.expires_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=?", hashToken(value)).Scan(&session.User.ID, &session.User.Username, &session.CSRFToken, &expires)
	if err != nil {
		return Session{}, err
	}
	session.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expires)
	if !session.ExpiresAt.After(time.Now().UTC()) {
		_, _ = s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash=?", hashToken(value))
		return Session{}, sql.ErrNoRows
	}
	_, _ = s.db.ExecContext(ctx, "UPDATE sessions SET last_seen_at=? WHERE token_hash=?", time.Now().UTC().Format(time.RFC3339Nano), hashToken(value))
	return session, nil
}
func (s *Service) Logout(ctx context.Context, value string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash=?", hashToken(value))
	return err
}

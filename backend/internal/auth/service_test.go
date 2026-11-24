package auth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/atlas/atlas/backend/internal/platform/database"
)

func TestBootstrapAndSessionLifecycle(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "atlas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := NewService(db)
	if err = s.Bootstrap(context.Background(), "admin", "a-safe-password"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Login(context.Background(), "admin", "wrong-password"); err == nil {
		t.Fatal("wrong password was accepted")
	}
	login, err := s.Login(context.Background(), "admin", "a-safe-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Session(context.Background(), login.Token); err != nil {
		t.Fatal(err)
	}
	if err = s.Logout(context.Background(), login.Token); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Session(context.Background(), login.Token); err == nil {
		t.Fatal("logged out session was accepted")
	}
}

package target

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/atlas/atlas/backend/internal/scope"
	"github.com/google/uuid"
)

type Service struct {
	db     *sql.DB
	repo   *Repository
	scopes *scope.Repository
}

func NewService(db *sql.DB, repo *Repository, scopes *scope.Repository) *Service {
	return &Service{db, repo, scopes}
}
func validate(in CreateInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if _, err := scope.NormalizeHost(in.PrimaryDomain); err != nil {
		return fmt.Errorf("primary domain: %w", err)
	}
	if in.ProgramURL != "" {
		u, e := url.ParseRequestURI(in.ProgramURL)
		if e != nil || u.Scheme != "https" && u.Scheme != "http" {
			return fmt.Errorf("program URL must be an http or https URL")
		}
	}
	if in.RateLimitPerMinute != nil && *in.RateLimitPerMinute <= 0 {
		return fmt.Errorf("rate limit must be positive")
	}
	return nil
}
func (s *Service) Create(ctx context.Context, in CreateInput, rules []scope.Input) (Target, error) {
	if err := validate(in); err != nil {
		return Target{}, err
	}
	domain, _ := scope.NormalizeHost(in.PrimaryDomain)
	now := time.Now().UTC()
	t := Target{ID: uuid.NewString(), Name: strings.TrimSpace(in.Name), PrimaryDomain: domain, Platform: strings.TrimSpace(in.Platform), ProgramURL: in.ProgramURL, Description: in.Description, ProgramRules: in.ProgramRules, ProgramRestrictions: in.ProgramRestrictions, RateLimitPerMinute: in.RateLimitPerMinute, Status: StatusActive, CreatedAt: now, UpdatedAt: now}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Target{}, err
	}
	defer tx.Rollback()
	if err = s.repo.Create(ctx, tx, t); err != nil {
		return Target{}, err
	}
	if err = s.scopes.CreateAll(ctx, tx, t.ID, rules); err != nil {
		return Target{}, err
	}
	if err = tx.Commit(); err != nil {
		return Target{}, err
	}
	return t, nil
}
func (s *Service) Get(ctx context.Context, id string) (Target, error) { return s.repo.Get(ctx, id) }
func (s *Service) List(ctx context.Context, status Status, limit int) ([]Target, error) {
	return s.repo.List(ctx, status, limit)
}
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (Target, error) {
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return t, err
	}
	if in.Name != nil {
		t.Name = strings.TrimSpace(*in.Name)
	}
	if in.Platform != nil {
		t.Platform = *in.Platform
	}
	if in.ProgramURL != nil {
		t.ProgramURL = *in.ProgramURL
	}
	if in.Description != nil {
		t.Description = *in.Description
	}
	if in.ProgramRules != nil {
		t.ProgramRules = *in.ProgramRules
	}
	if in.ProgramRestrictions != nil {
		t.ProgramRestrictions = *in.ProgramRestrictions
	}
	if in.RateLimitPerMinute != nil {
		if *in.RateLimitPerMinute <= 0 {
			return t, fmt.Errorf("rate limit must be positive")
		}
		t.RateLimitPerMinute = in.RateLimitPerMinute
	}
	if in.Status != nil {
		if *in.Status != StatusActive && *in.Status != StatusArchived {
			return t, fmt.Errorf("invalid target status")
		}
		t.Status = *in.Status
		if t.Status == StatusArchived {
			n := time.Now().UTC()
			t.ArchivedAt = &n
		} else {
			t.ArchivedAt = nil
		}
	}
	if t.Name == "" {
		return t, fmt.Errorf("name is required")
	}
	if t.ProgramURL != "" {
		if _, e := url.ParseRequestURI(t.ProgramURL); e != nil {
			return t, fmt.Errorf("invalid program URL")
		}
	}
	t.UpdatedAt = time.Now().UTC()
	if err = s.repo.Update(ctx, t); err != nil {
		return t, err
	}
	return t, nil
}
func (s *Service) Counts(ctx context.Context, id string) (int, int, int, error) {
	return s.repo.Counts(ctx, id)
}

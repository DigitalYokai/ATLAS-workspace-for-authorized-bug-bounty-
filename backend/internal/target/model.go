package target

import "time"

type Status string

const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

type Target struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	PrimaryDomain       string     `json:"primaryDomain"`
	Platform            string     `json:"platform"`
	ProgramURL          string     `json:"programUrl"`
	Description         string     `json:"description"`
	ProgramRules        string     `json:"programRules"`
	ProgramRestrictions string     `json:"programRestrictions"`
	RateLimitPerMinute  *int       `json:"rateLimitPerMinute,omitempty"`
	Status              Status     `json:"status"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	ArchivedAt          *time.Time `json:"archivedAt,omitempty"`
}

type CreateInput struct {
	Name                string `json:"name"`
	PrimaryDomain       string `json:"primaryDomain"`
	Platform            string `json:"platform"`
	ProgramURL          string `json:"programUrl"`
	Description         string `json:"description"`
	ProgramRules        string `json:"programRules"`
	ProgramRestrictions string `json:"programRestrictions"`
	RateLimitPerMinute  *int   `json:"rateLimitPerMinute"`
}
type UpdateInput struct {
	Name                *string `json:"name"`
	Platform            *string `json:"platform"`
	ProgramURL          *string `json:"programUrl"`
	Description         *string `json:"description"`
	ProgramRules        *string `json:"programRules"`
	ProgramRestrictions *string `json:"programRestrictions"`
	RateLimitPerMinute  *int    `json:"rateLimitPerMinute"`
	Status              *Status `json:"status"`
}

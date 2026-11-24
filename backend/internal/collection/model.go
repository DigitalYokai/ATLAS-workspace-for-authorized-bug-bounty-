package collection

import "time"

type Mode string

const (
	Passive Mode = "passive"
	Active  Mode = "active"
)

type Status string

const (
	Pending            Status = "pending"
	Running            Status = "running"
	Completed          Status = "completed"
	PartiallyCompleted Status = "partially_completed"
	Failed             Status = "failed"
	Cancelled          Status = "cancelled"
)

type Configuration struct {
	Concurrency        int `json:"concurrency"`
	TimeoutSeconds     int `json:"timeoutSeconds"`
	RateLimitPerMinute int `json:"rateLimitPerMinute"`
}
type CreateInput struct {
	Collectors    []string      `json:"collectors"`
	Mode          Mode          `json:"mode"`
	Configuration Configuration `json:"configuration"`
}
type Run struct {
	ID            string        `json:"id"`
	TargetID      string        `json:"targetId"`
	Mode          Mode          `json:"mode"`
	Status        Status        `json:"status"`
	Configuration Configuration `json:"configuration"`
	ErrorSummary  string        `json:"errorSummary"`
	TotalJobs     int           `json:"totalJobs"`
	CompletedJobs int           `json:"completedJobs"`
	FailedJobs    int           `json:"failedJobs"`
	StartedAt     *time.Time    `json:"startedAt,omitempty"`
	CompletedAt   *time.Time    `json:"completedAt,omitempty"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}
type Job struct {
	ID            string     `json:"id"`
	CollectorName string     `json:"collectorName"`
	Status        Status     `json:"status"`
	ErrorSummary  string     `json:"errorSummary"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
}
type Event struct {
	ID        string    `json:"id"`
	EventType string    `json:"eventType"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

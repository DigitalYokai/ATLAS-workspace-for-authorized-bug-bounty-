package scope

import "time"

type Kind string

const (
	Include Kind = "include"
	Exclude Kind = "exclude"
)

type PatternKind string

const (
	Exact    PatternKind = "exact"
	Wildcard PatternKind = "wildcard"
)

type Rule struct {
	ID               string      `json:"id"`
	TargetID         string      `json:"targetId"`
	Kind             Kind        `json:"kind"`
	Pattern          string      `json:"pattern"`
	PatternKind      PatternKind `json:"patternKind"`
	PermittedMethods []string    `json:"permittedMethods"`
	CreatedAt        time.Time   `json:"createdAt"`
	UpdatedAt        time.Time   `json:"updatedAt"`
}
type Input struct {
	Kind             Kind     `json:"kind"`
	Pattern          string   `json:"pattern"`
	PermittedMethods []string `json:"permittedMethods"`
}
type Decision struct {
	Host           string `json:"host"`
	Method         string `json:"method"`
	Status         string `json:"status"`
	ActiveApproved bool   `json:"activeApproved"`
	Reason         string `json:"reason"`
	MatchedRuleID  string `json:"matchedRuleId,omitempty"`
}

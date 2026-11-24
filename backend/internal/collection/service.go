package collection

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/atlas/atlas/backend/internal/observation"
	"github.com/atlas/atlas/backend/internal/scope"
	"github.com/atlas/atlas/backend/internal/target"
	"sync"
	"time"
)

type Service struct {
	repo       *Repository
	targets    *target.Repository
	scopes     *scope.Repository
	ingest     *observation.Ingestor
	collectors map[string]Collector
	globalSem  chan struct{}
	mu         sync.Mutex
	cancels    map[string]context.CancelFunc
}

func NewService(db *sql.DB, collectors []Collector) *Service {
	sr := scope.NewRepository(db)
	svc := &Service{repo: NewRepository(db), targets: target.NewRepository(db), scopes: sr, ingest: observation.NewIngestor(db, sr), collectors: func() map[string]Collector {
		m := map[string]Collector{}
		for _, c := range collectors {
			m[c.Name()] = c
		}
		return m
	}(), globalSem: make(chan struct{}, 4), cancels: map[string]context.CancelFunc{}}
	if err := svc.repo.RecoverInterrupted(context.Background()); err == nil {
		go svc.resumePending()
	}
	return svc
}

// resumePending restores work that was interrupted by a process shutdown. Runs
// are claimed before execution, making this safe if more than one ATLAS process
// happens to start against the same database.
func (s *Service) resumePending() {
	ctx := context.Background()
	runs, err := s.repo.Pending(ctx)
	if err != nil {
		return
	}
	for _, run := range runs {
		t, err := s.targets.Get(ctx, run.TargetID)
		if err != nil {
			continue
		}
		rules, err := s.scopes.List(ctx, run.TargetID)
		if err != nil {
			continue
		}
		go s.execute(run, t.PrimaryDomain, rules)
	}
}
func (s *Service) Start(ctx context.Context, targetID string, in CreateInput) (Run, error) {
	if in.Mode == "" {
		in.Mode = Passive
	}
	if in.Mode != Passive && in.Mode != Active {
		return Run{}, fmt.Errorf("invalid collection mode")
	}
	if len(in.Collectors) == 0 {
		return Run{}, fmt.Errorf("select at least one collector")
	}
	if in.Configuration.Concurrency < 1 || in.Configuration.Concurrency > 4 {
		in.Configuration.Concurrency = 2
	}
	if in.Configuration.TimeoutSeconds < 1 || in.Configuration.TimeoutSeconds > 60 {
		in.Configuration.TimeoutSeconds = 15
	}
	t, e := s.targets.Get(ctx, targetID)
	if e != nil {
		return Run{}, e
	}
	if in.Configuration.RateLimitPerMinute < 1 {
		if t.RateLimitPerMinute != nil {
			in.Configuration.RateLimitPerMinute = *t.RateLimitPerMinute
		} else {
			in.Configuration.RateLimitPerMinute = 30
		}
	}
	rules, e := s.scopes.List(ctx, targetID)
	if e != nil {
		return Run{}, e
	}
	for _, name := range in.Collectors {
		c, ok := s.collectors[name]
		if !ok {
			return Run{}, fmt.Errorf("unknown collector %q", name)
		}
		if c.Active() {
			if in.Mode != Active {
				return Run{}, fmt.Errorf("collector %q requires active mode", name)
			}
			method := "http_probe"
			if active, ok := c.(ActiveCollector); ok {
				method = active.ScopeMethod()
			}
			if !scope.Evaluate(rules, t.PrimaryDomain, method).ActiveApproved {
				return Run{}, fmt.Errorf("active collection denied by scope")
			}
		}
	}
	run, e := s.repo.Create(ctx, targetID, in)
	if e != nil {
		return Run{}, e
	}
	go s.execute(run, t.PrimaryDomain, rules)
	return run, nil
}
func (s *Service) execute(run Run, domain string, rules []scope.Rule) {
	ctx, cancel := context.WithCancel(context.Background())
	claimed, err := s.repo.ClaimRun(ctx, run.ID)
	if err != nil || !claimed {
		cancel()
		return
	}
	s.mu.Lock()
	s.cancels[run.ID] = cancel
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.cancels, run.ID); s.mu.Unlock() }()
	_ = s.repo.AddEvent(ctx, run.TargetID, run.ID, "run_started", "Collection run started")
	jobs, err := s.repo.Jobs(ctx, run.ID)
	if err != nil {
		_ = s.repo.SetRun(context.Background(), run.ID, Failed, 0, 1, "load collection jobs: "+err.Error())
		return
	}
	sem := make(chan struct{}, run.Configuration.Concurrency)
	var rateTicker *time.Ticker
	var rateGate <-chan time.Time
	if run.Mode == Active && run.Configuration.RateLimitPerMinute > 0 {
		interval := time.Minute / time.Duration(run.Configuration.RateLimitPerMinute)
		rateTicker = time.NewTicker(interval)
		rateGate = rateTicker.C
		defer rateTicker.Stop()
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, job := range jobs {
		if job.Status != Pending {
			continue
		}
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			s.globalSem <- struct{}{}
			defer func() { <-s.globalSem }()
			if ctx.Err() != nil {
				_ = s.repo.SetJob(ctx, job.ID, Cancelled, "cancelled")
				return
			}
			_ = s.repo.SetJob(ctx, job.ID, Running, "")
			c := s.collectors[job.CollectorName]
			if c.Active() && rateGate != nil {
				select {
				case <-ctx.Done():
					return
				case <-rateGate:
				}
			}
			cctx, stop := context.WithTimeout(ctx, time.Duration(run.Configuration.TimeoutSeconds)*time.Second)
			err := c.Collect(cctx, Request{TargetID: run.TargetID, PrimaryDomain: domain, Mode: run.Mode, Rules: rules, Configuration: run.Configuration}, func(o Observation) error {
				return s.ingest.Ingest(cctx, run.TargetID, run.ID, observation.Input{EntityType: o.EntityType, Value: o.Value, Source: o.Source, Confidence: o.Confidence, Metadata: o.Metadata, Evidence: o.Evidence, ObservedAt: o.ObservedAt})
			})
			stop()
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				_ = s.repo.SetJob(ctx, job.ID, Failed, err.Error())
				_ = s.repo.AddEvent(ctx, run.TargetID, run.ID, "job_failed", job.CollectorName+": "+err.Error())
			} else {
				_ = s.repo.SetJob(ctx, job.ID, Completed, "")
				_ = s.repo.AddEvent(ctx, run.TargetID, run.ID, "job_completed", job.CollectorName)
			}
		}()
	}
	wg.Wait()
	jobs, err = s.repo.Jobs(context.Background(), run.ID)
	if err != nil {
		return
	}
	done, failed := 0, 0
	for _, job := range jobs {
		switch job.Status {
		case Completed:
			done++
		case Failed:
			failed++
		}
	}
	status := Completed
	if ctx.Err() != nil {
		status = Cancelled
	} else if failed > 0 && done > 0 {
		status = PartiallyCompleted
	} else if failed > 0 {
		status = Failed
	}
	_ = s.repo.SetRun(context.Background(), run.ID, status, done, failed, "")
	_ = s.repo.AddEvent(context.Background(), run.TargetID, run.ID, "run_finished", string(status))
}
func (s *Service) Cancel(id string) bool {
	s.mu.Lock()
	c, ok := s.cancels[id]
	if ok {
		c()
	}
	s.mu.Unlock()
	persisted, err := s.repo.Cancel(context.Background(), id)
	return err == nil && (ok || persisted)
}
func (s *Service) Get(ctx context.Context, id string) (Run, error) { return s.repo.Get(ctx, id) }
func (s *Service) List(ctx context.Context, targetID string) ([]Run, error) {
	return s.repo.List(ctx, targetID)
}
func (s *Service) Events(ctx context.Context, id, after string) ([]Event, error) {
	return s.repo.Events(ctx, id, after)
}

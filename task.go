// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style license.

// task.go — task model, in-memory store with memory-store persistence, ticker.
package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Task statuses
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusDone      = "done"
	StatusCancelled = "cancelled"
	StatusFailed    = "failed"
)

// Task is a scheduled task for an AI agent.
type Task struct {
	ID        string    `json:"id"`
	When      string    `json:"when"`   // ISO 8601 absolute time
	WhenTime  time.Time `json:"-"`      // parsed When (UTC)
	Every     string    `json:"every"`  // recurring period, e.g. "24h" (empty = one-off)
	EveryDur  time.Duration `json:"-"`  // parsed Every
	Action    string    `json:"action"` // free-text prompt for the agent
	Author    string    `json:"author"`
	Source    string    `json:"source"`
	Agent     string    `json:"agent"`
	Model     string    `json:"model,omitempty"`
	Expired   string    `json:"expired"` // run | skip (one-off overdue policy)
	Status    string    `json:"status"`
	Result    string    `json:"result,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TaskUpdate is a partial update for a task.
type TaskUpdate struct {
	When   *string
	Action *string
	Every  *string
}

// Scheduler is the in-memory task store with a ticker.
type Scheduler struct {
	mu       sync.Mutex
	tasks    map[string]*Task
	every    time.Duration // ticker interval
	sem      chan struct{} // bounded concurrency
	executor *executor
}

// SchedulerInstance is the global scheduler used by MCP tools.
var SchedulerInstance = NewScheduler(10 * time.Second)

// maxConcurrent is the default limit of simultaneously running tasks.
const maxConcurrent = 3

// scheduler is a shorthand used by serve.go.
var scheduler = SchedulerInstance

// NewScheduler creates a scheduler with the given tick interval.
func NewScheduler(tick time.Duration) *Scheduler {
	s := &Scheduler{
		tasks:    map[string]*Task{},
		every:    tick,
		sem:      make(chan struct{}, maxConcurrent),
		executor: newExecutor(),
	}
	s.Start()
	return s
}

// Start launches the ticker goroutine that checks due tasks.
func (s *Scheduler) Start() {
	go func() {
		for {
			time.Sleep(s.every)
			s.checkDue()
		}
	}()
}

// Create adds a new task.
func (s *Scheduler) Create(t Task) (*Task, error) {
	whenTime, err := parseWhen(t.When)
	if err != nil {
		return nil, err
	}
	t.ID = uuid.NewString()
	t.WhenTime = whenTime
	t.Status = StatusPending
	t.CreatedAt = time.Now().UTC()
	t.UpdatedAt = t.CreatedAt
	if t.Agent == "" {
		t.Agent = "baron"
	}
	if t.Author == "" {
		t.Author = "kirill"
	}
	if t.Expired == "" {
		t.Expired = "run"
	}
	if t.Every != "" {
		d, err := time.ParseDuration(t.Every)
		if err != nil {
			return nil, fmt.Errorf("invalid every duration %q: %w", t.Every, err)
		}
		t.EveryDur = d
	}

	s.mu.Lock()
	s.tasks[t.ID] = &t
	s.mu.Unlock()

	s.persist()
	return &t, nil
}

// List returns tasks, optionally filtered by status.
func (s *Scheduler) List(status string) []*Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Task
	for _, t := range s.tasks {
		if status == "" || t.Status == status {
			out = append(out, t)
		}
	}
	return out
}

// Cancel marks a task as cancelled.
func (s *Scheduler) Cancel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}
	t.Status = StatusCancelled
	t.UpdatedAt = time.Now().UTC()
	s.persistLocked()
	return nil
}

// Update partially updates a task.
func (s *Scheduler) Update(id string, upd TaskUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}
	if upd.When != nil && *upd.When != "" {
		whenTime, err := parseWhen(*upd.When)
		if err != nil {
			return err
		}
		t.When = *upd.When
		t.WhenTime = whenTime
	}
	if upd.Action != nil && *upd.Action != "" {
		t.Action = *upd.Action
	}
	if upd.Every != nil {
		if *upd.Every == "" {
			t.Every = ""
			t.EveryDur = 0
		} else {
			d, err := time.ParseDuration(*upd.Every)
			if err != nil {
				return fmt.Errorf("invalid every duration: %w", err)
			}
			t.Every = *upd.Every
			t.EveryDur = d
		}
	}
	t.UpdatedAt = time.Now().UTC()
	s.persistLocked()
	return nil
}

// checkDue finds due tasks and triggers execution.
func (s *Scheduler) checkDue() {
	now := time.Now().UTC()
	s.mu.Lock()
	var due []*Task
	for _, t := range s.tasks {
		if t.Status != StatusPending {
			continue
		}
		if !now.Before(t.WhenTime) {
			due = append(due, t)
		}
	}
	for _, t := range due {
		t.Status = StatusRunning
		t.UpdatedAt = now
	}
	s.mu.Unlock()

	for _, t := range due {
		s.execute(t)
	}
}

// execute runs a due task via the opencode executor (bounded concurrency).
func (s *Scheduler) execute(t *Task) {
	// Bounded concurrency: at most maxConcurrent tasks run at once.
	s.sem <- struct{}{}
	go func() {
		defer func() { <-s.sem }()

		result, err := s.executor.run(t)
		s.mu.Lock()
		if err != nil {
			t.Status = StatusFailed
			t.Result = "error: " + err.Error()
		} else {
			t.Status = StatusDone
			t.Result = result
		}
		t.UpdatedAt = time.Now().UTC()
		s.mu.Unlock()
		s.persist()

		// Notify Kirill in his opencode chat about completion/failure.
		status := "выполнена"
		if err != nil {
			status = "упала"
		}
		notify("scheduler", fmt.Sprintf("Задача %s %s: %s", t.ID[:8], status, truncate(t.Action, 60)))
	}()
}

// persist saves tasks to memory-store (memory/scheduler/tasks).
// TODO: implement via memory-store-mcp HTTP API.
func (s *Scheduler) persist() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.persistLocked()
}

func (s *Scheduler) persistLocked() {
	// TODO: write tasks snapshot to memory-store
}

// parseWhen parses an ISO 8601 time.
func parseWhen(when string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, when)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid when %q: must be ISO 8601 (e.g. 2026-08-04T10:00:00Z)", when)
	}
	return t.UTC(), nil
}

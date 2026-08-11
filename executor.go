// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style license.

// executor.go — task execution via opencode-client (HTTP).
//
// When a task is due, the scheduler spawns an opencode session (agent)
// with the task's action prompt and waits for the reply. The result is
// stored on the task and, if configured, a notification is sent to
// Kirill's opencode chat (HTTP :7799/notify) and/or Discord.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	opencodeclient "github.com/kirill-scherba/opencode-client"
)

const (
	// opencodeBaseURL is the opencode-serve HTTP server.
	opencodeBaseURL = "http://127.0.0.1:7709"
	// notifyURL is the notifications.ts plugin HTTP endpoint.
	notifyURL = "http://127.0.0.1:7799/notify"
	// defaultTaskTimeout bounds a single task execution.
	defaultTaskTimeout = 10 * time.Minute
)

// executor runs due tasks via opencode-client.
type executor struct {
	client *opencodeclient.Client
}

// newExecutor creates the executor with the default opencode client.
func newExecutor() *executor {
	return &executor{
		client: opencodeclient.New(opencodeBaseURL, defaultTaskTimeout),
	}
}

// run executes a task: creates/reuses a session, sends the prompt, waits.
// It returns the agent reply (or an error).
func (e *executor) run(t *Task) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTaskTimeout)
	defer cancel()

	// Session name: stable per task, allows reuse across retries.
	sessionName := "sched-" + t.ID[:8]
	sess, err := e.client.CreateSession(sessionName, t.Agent, t.Model, "", "scheduler", nil)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	// Always release the session when done — including SendMessage errors
	// and timeouts — so recurring tasks do not accumulate sessions in
	// opencode-serve (DELETE /session/{id}).
	defer e.client.CloseSessionID(sess.ID)

	// Send the action prompt and wait for the reply. The client's
	// SendMessage has its own internal timeout; wrap with ctx so the
	// caller can cancel earlier if needed.
	type reply struct {
		text string
		err  error
	}
	respCh := make(chan reply, 1)
	go func() {
		text, err := e.client.SendMessage(sess, t.Action)
		respCh <- reply{text: text, err: err}
	}()

	select {
	case <-ctx.Done():
		// The deferred CloseSessionID releases the session on timeout.
		return "", fmt.Errorf("task timeout: %w", ctx.Err())
	case r := <-respCh:
		if r.err != nil {
			return "", fmt.Errorf("send message: %w", r.err)
		}
		return r.text, nil
	}
}

// notify sends a message to Kirill's opencode chat via the plugin HTTP
// endpoint. Errors are logged but never block the scheduler.
func notify(source, text string) {
	payload := map[string]string{"source": source, "text": text}
	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "notify: marshal: %v\n", err)
		return
	}
	resp, err := http.Post(notifyURL, "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "notify: POST %s: %v\n", notifyURL, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "notify: status %d\n", resp.StatusCode)
	}
}

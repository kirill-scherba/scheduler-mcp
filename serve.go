// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style license.

// serve.go — MCP server (stdio) for scheduler-mcp.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run MCP server (stdio or HTTP)",
		Long:  "Start MCP server for AI agents using mcp-go. Use --http to enable StreamableHTTP transport.",
		RunE: func(cmd *cobra.Command, args []string) error {
			httpAddr, _ := cmd.Flags().GetString("http")
			return serveMCP(httpAddr)
		},
	}
	cmd.Flags().String("http", "", "HTTP listen address (enables StreamableHTTP transport, e.g. ':7713')")
	return cmd
}

func serveMCP(httpAddr string) error {
	s := server.NewMCPServer(
		"scheduler-mcp",
		"0.1.0",
		server.WithInstructions("Universal scheduled tasks for AI agents. Create one-off and recurring tasks that run at a specific time via opencode sessions. A skill documents how to formulate tasks."),
	)

	s.AddTools(schedulerTools()...)

	// ── MCP Resources ──────────────────────────────────────────────────────
	// Human-friendly @-menu access: Kirill can insert live task list and
	// statistics into the chat without formulating a query.

	// scheduler://tasks — full task list
	tasksRes := mcp.NewResource("scheduler://tasks",
		"Scheduled Tasks",
		mcp.WithResourceDescription("List of scheduled tasks with status, time, author and result"),
		mcp.WithMIMEType("application/json"),
	)
	s.AddResource(tasksRes, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		tasks := scheduler.List("")
		type taskView struct {
			ID        string `json:"id"`
			When      string `json:"when"`
			Every     string `json:"every,omitempty"`
			Status    string `json:"status"`
			Agent     string `json:"agent,omitempty"`
			Author    string `json:"author"`
			Action    string `json:"action"`
			Result    string `json:"result,omitempty"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
		}
		views := make([]taskView, 0, len(tasks))
		for _, t := range tasks {
			views = append(views, taskView{
				ID: t.ID, When: t.When, Every: t.Every, Status: t.Status,
				Agent: t.Agent, Author: t.Author, Action: t.Action, Result: t.Result,
				CreatedAt: t.CreatedAt.Format(time.RFC3339),
				UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
			})
		}
		data, _ := json.MarshalIndent(views, "", "  ")
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "scheduler://tasks",
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	})

	// scheduler://stats — execution statistics
	statsRes := mcp.NewResource("scheduler://stats",
		"Scheduler Stats",
		mcp.WithResourceDescription("Scheduler execution statistics: totals by status"),
		mcp.WithMIMEType("application/json"),
	)
	s.AddResource(statsRes, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		tasks := scheduler.List("")
		stats := map[string]int{}
		for _, t := range tasks {
			stats[t.Status]++
		}
		stats["total"] = len(tasks)
		data, _ := json.MarshalIndent(stats, "", "  ")
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "scheduler://stats",
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	})

	if httpAddr != "" {
		log.Printf("Starting scheduler-mcp StreamableHTTP on %s", httpAddr)
		httpServer := server.NewStreamableHTTPServer(s, server.WithStateLess(true))
		return httpServer.Start(httpAddr)
	}
	return server.ServeStdio(s)
}

func schedulerTools() []server.ServerTool {
	tools := []server.ServerTool{}

	// scheduler_create
	createTool := mcp.NewTool("scheduler_create",
		mcp.WithDescription("Create a scheduled task for an AI agent. The task runs at the given time (or periodically) by spawning an agent with the action prompt. Author/source provenance is stored for safety."),
		mcp.WithString("when",
			mcp.Description("When to run: ISO 8601 time (2026-08-04T10:00:00Z) or natural language handled by the agent (10:00, tomorrow 10:00, in 30 minutes) — recommend passing absolute ISO time"),
		),
		mcp.WithString("action",
			mcp.Description("Free-text prompt for the agent to execute (required)"),
		),
		mcp.WithString("author",
			mcp.Description("Who created the task: kirill, agent:baron, email:<addr>, etc. Default: kirill"),
		),
		mcp.WithString("source",
			mcp.Description("Where the task came from: user_session, mail_session, telegram, etc."),
		),
		mcp.WithString("agent",
			mcp.Description("Which agent executes (default: baron)"),
		),
		mcp.WithString("every",
			mcp.Description("Recurring period, e.g. 24h, 7d (optional). When set, task repeats"),
		),
		mcp.WithString("expired",
			mcp.Description("Overdue policy for one-off tasks: run (default) or skip"),
		),
	)
	tools = append(tools, server.ServerTool{
		Tool: createTool,
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()
			action, _ := args["action"].(string)
			if action == "" {
				return mcp.NewToolResultError("'action' is required"), nil
			}
			when, _ := args["when"].(string)
			if when == "" {
				return mcp.NewToolResultError("'when' is required"), nil
			}

			task, err := scheduler.Create(Task{
				When:    when,
				Action:  action,
				Author:  str(args, "author", "kirill"),
				Source:  str(args, "source", "user_session"),
				Agent:   str(args, "agent", "baron"),
				Every:   str(args, "every", ""),
				Expired: str(args, "expired", "run"),
			})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Task created: id=%s when=%s action=%q author=%s",
				task.ID, task.When, task.Action, task.Author)), nil
		},
	})

	// scheduler_list
	listTool := mcp.NewTool("scheduler_list",
		mcp.WithDescription("List scheduled tasks, optionally filtered by status"),
		mcp.WithString("status",
			mcp.Description("Filter: pending, running, done, cancelled, failed (empty = all)"),
		),
	)
	tools = append(tools, server.ServerTool{
		Tool: listTool,
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()
			status, _ := args["status"].(string)
			tasks := scheduler.List(status)
			if len(tasks) == 0 {
				return mcp.NewToolResultText("No tasks found"), nil
			}
			var sb strings.Builder
			for _, t := range tasks {
				sb.WriteString(fmt.Sprintf("%s  %s  %s  [%s]  %s\n",
					t.ID[:8], t.When, t.Status, t.Author, truncate(t.Action, 60)))
				if t.Result != "" && t.Result != "executed (stub)" {
					sb.WriteString(fmt.Sprintf("        → %s\n", truncate(t.Result, 120)))
				}
			}
			return mcp.NewToolResultText(sb.String()), nil
		},
	})

	// scheduler_cancel
	cancelTool := mcp.NewTool("scheduler_cancel",
		mcp.WithDescription("Cancel a scheduled task by id"),
		mcp.WithString("id",
			mcp.Description("Task id (required)"),
		),
	)
	tools = append(tools, server.ServerTool{
		Tool: cancelTool,
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()
			id, _ := args["id"].(string)
			if id == "" {
				return mcp.NewToolResultError("'id' is required"), nil
			}
			if err := scheduler.Cancel(id); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText("Task cancelled: " + id), nil
		},
	})

	// scheduler_update
	updateTool := mcp.NewTool("scheduler_update",
		mcp.WithDescription("Update a scheduled task (when, action, every)"),
		mcp.WithString("id",
			mcp.Description("Task id (required)"),
		),
		mcp.WithString("when",
			mcp.Description("New time (ISO 8601)"),
		),
		mcp.WithString("action",
			mcp.Description("New action prompt"),
		),
		mcp.WithString("every",
			mcp.Description("New recurring period (empty = remove)"),
		),
	)
	tools = append(tools, server.ServerTool{
		Tool: updateTool,
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.GetArguments()
			id, _ := args["id"].(string)
			if id == "" {
				return mcp.NewToolResultError("'id' is required"), nil
			}
			upd := TaskUpdate{
				When:   strPtr(args, "when"),
				Action: strPtr(args, "action"),
				Every:  strPtr(args, "every"),
			}
			if err := scheduler.Update(id, upd); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText("Task updated: " + id), nil
		},
	})

	// scheduler_stats
	statsTool := mcp.NewTool("scheduler_stats",
		mcp.WithDescription("Scheduler execution statistics: totals by status, total runs, average task duration"),
	)
	tools = append(tools, server.ServerTool{
		Tool: statsTool,
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			tasks := scheduler.List("")
			byStatus := map[string]int{}
			totalRuns := 0
			for _, t := range tasks {
				byStatus[t.Status]++
				totalRuns += t.Runs
			}
			// Average duration for finished tasks (done/failed).
			var avgMs, n int64
			for _, t := range tasks {
				if t.Status == StatusDone || t.Status == StatusFailed {
					d := t.UpdatedAt.Sub(t.CreatedAt).Milliseconds()
					if d > 0 {
						avgMs += d
						n++
					}
				}
			}
			if n > 0 {
				avgMs /= n
			}
			stats := map[string]any{
				"total":           len(tasks),
				"by_status":       byStatus,
				"total_runs":      totalRuns,
				"avg_duration_ms": avgMs,
			}
			data, _ := json.MarshalIndent(stats, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	})

	return tools
}

func str(args map[string]any, key, def string) string {
	if v, ok := args[key].(string); ok && v != "" {
		return v
	}
	return def
}

func strPtr(args map[string]any, key string) *string {
	if v, ok := args[key].(string); ok {
		return &v
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

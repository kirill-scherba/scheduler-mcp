// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style license.

// scheduler-mcp — universal scheduled tasks for AI agents.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	log.SetFlags(0)

	rootCmd := &cobra.Command{
		Use:   "scheduler-mcp",
		Short: "Universal scheduled tasks for AI agents",
		Long:  "scheduler-mcp — one-off and recurring tasks for AI agents, executed at a specific time via opencode sessions.",
	}

	rootCmd.AddCommand(serveCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

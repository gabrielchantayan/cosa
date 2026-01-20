// Command cosa is the Cosa CLI client.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"cosa/internal/config"
	"cosa/internal/daemon"
	"cosa/internal/protocol"
	"cosa/internal/tui"
)

var cfg *config.Config

func main() {
	var err error
	cfg, err = config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	rootCmd := &cobra.Command{
		Use:   "cosa",
		Short: "Cosa Nostra - Multi-agent development orchestration",
		Long: `Cosa Nostra is a mafia-themed multi-agent development orchestration system.
It manages Claude Code workers in isolated git worktrees with a hierarchical
role system and real-time TUI.`,
	}

	rootCmd.AddCommand(
		startCmd(),
		stopCmd(),
		statusCmd(),
		versionCmd(),
		territoryCmd(),
		workerCmd(),
		jobCmd(),
		reviewCmd(),
		operationCmd(),
		orderCmd(),
		logsCmd(),
		settingsCmd(),
		tuiCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func startCmd() *cobra.Command {
	var foreground bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Cosa daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemon.IsRunning(cfg.SocketPath) {
				fmt.Println("Daemon is already running")
				return nil
			}

			if foreground {
				return runDaemonForeground()
			}

			return startDaemonBackground()
		},
	}

	cmd.Flags().BoolVarP(&foreground, "foreground", "f", false, "Run daemon in foreground")

	return cmd
}

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the Cosa daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				fmt.Println("Daemon is not running")
				return nil
			}
			defer client.Close()

			if err := client.Shutdown(); err != nil {
				return fmt.Errorf("failed to stop daemon: %w", err)
			}

			fmt.Println("Daemon stopped")
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				fmt.Println("Daemon is not running")
				return nil
			}
			defer client.Close()

			status, err := client.Status()
			if err != nil {
				return fmt.Errorf("failed to get status: %w", err)
			}

			fmt.Printf("Cosa Daemon v%s\n", status.Version)
			fmt.Printf("Status:      running\n")
			fmt.Printf("Uptime:      %s\n", formatDuration(time.Duration(status.Uptime)*time.Second))
			fmt.Printf("Workers:     %d\n", status.Workers)
			fmt.Printf("Active Jobs: %d\n", status.ActiveJobs)
			if status.Territory != "" {
				fmt.Printf("Territory:   %s\n", status.Territory)
			}
			if status.TotalCost != "" && status.TotalCost != "$0.00" {
				fmt.Printf("Total Cost:  %s (%d tokens)\n", status.TotalCost, status.TotalTokens)
			}

			return nil
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("cosa version %s\n", config.Version)
		},
	}
}

// Territory commands

func territoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "territory",
		Short: "Manage the Cosa territory (workspace)",
	}

	cmd.AddCommand(
		territoryInitCmd(),
		territoryStatusCmd(),
		territoryListCmd(),
		territoryAddCmd(),
	)

	return cmd
}

func territoryInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new territory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := connectOrStartDaemon()
			if err != nil {
				return err
			}
			defer client.Close()

			path := ""
			if len(args) > 0 {
				path = args[0]
			}

			resp, err := client.Call(protocol.MethodTerritoryInit, map[string]string{"path": path})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var result map[string]string
			json.Unmarshal(resp.Result, &result)
			fmt.Printf("Territory initialized at %s\n", result["path"])

			return nil
		},
	}
}

func territoryStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show territory status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodTerritoryStatus, nil)
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var result map[string]interface{}
			json.Unmarshal(resp.Result, &result)

			fmt.Printf("Path:        %s\n", result["path"])
			fmt.Printf("Repo Root:   %s\n", result["repo_root"])
			fmt.Printf("Base Branch: %s\n", result["base_branch"])

			return nil
		},
	}
}

func territoryListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all registered territories",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodTerritoryList, nil)
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var result struct {
				Territories []struct {
					Path       string `json:"path"`
					RepoRoot   string `json:"repo_root"`
					BaseBranch string `json:"base_branch"`
					Active     bool   `json:"active"`
				} `json:"territories"`
			}
			json.Unmarshal(resp.Result, &result)

			if len(result.Territories) == 0 {
				fmt.Println("No registered territories")
				return nil
			}

			fmt.Printf("%-50s %-12s %s\n", "PATH", "BRANCH", "STATUS")
			for _, t := range result.Territories {
				status := ""
				if t.Active {
					status = "active"
				}
				path := t.Path
				if len(path) > 48 {
					path = "..." + path[len(path)-45:]
				}
				fmt.Printf("%-50s %-12s %s\n", path, t.BaseBranch, status)
			}

			return nil
		},
	}
}

func territoryAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <path>",
		Short: "Register an existing project as a territory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := connectOrStartDaemon()
			if err != nil {
				return err
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodTerritoryAdd, map[string]string{"path": args[0]})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var result map[string]string
			json.Unmarshal(resp.Result, &result)
			fmt.Printf("Territory registered: %s\n", result["path"])

			return nil
		},
	}
}

// Worker commands

func workerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Manage workers",
	}

	cmd.AddCommand(
		workerAddCmd(),
		workerListCmd(),
		workerRemoveCmd(),
		workerMessageCmd(),
		workerHandoffCmd(),
		workerDetailCmd(),
	)

	return cmd
}

func workerAddCmd() *cobra.Command {
	var role string

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new worker",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			params := protocol.WorkerAddParams{
				Name: args[0],
				Role: role,
			}

			resp, err := client.Call(protocol.MethodWorkerAdd, params)
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var info protocol.WorkerInfo
			json.Unmarshal(resp.Result, &info)

			fmt.Printf("Worker added:\n")
			fmt.Printf("  Name:     %s\n", info.Name)
			fmt.Printf("  Role:     %s\n", info.Role)
			fmt.Printf("  Status:   %s\n", info.Status)
			fmt.Printf("  Worktree: %s\n", info.Worktree)

			return nil
		},
	}

	cmd.Flags().StringVarP(&role, "role", "r", "soldato", "Worker role (soldato, capo, consigliere)")

	return cmd
}

func workerListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all workers",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodWorkerList, nil)
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var workers []protocol.WorkerInfo
			json.Unmarshal(resp.Result, &workers)

			if len(workers) == 0 {
				fmt.Println("No workers")
				return nil
			}

			fmt.Printf("%-15s %-12s %-10s %s\n", "NAME", "ROLE", "STATUS", "CURRENT JOB")
			for _, w := range workers {
				job := "-"
				if w.CurrentJob != "" {
					job = w.CurrentJob[:8]
				}
				fmt.Printf("%-15s %-12s %-10s %s\n", w.Name, w.Role, w.Status, job)
			}

			return nil
		},
	}
}

func workerRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a worker",
		Aliases: []string{"rm"},
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodWorkerRemove, map[string]interface{}{
				"name":  args[0],
				"force": force,
			})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			fmt.Printf("Worker '%s' removed\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force removal even if working")

	return cmd
}

func workerMessageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "message <name> <text>",
		Short: "Send a message to a worker",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodWorkerMessage, map[string]string{
				"name":    args[0],
				"message": args[1],
			})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			fmt.Printf("Message sent to worker '%s'\n", args[0])
			return nil
		},
	}
}

func workerHandoffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "handoff <name>",
		Short: "Generate handoff summary for a worker",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodHandoffGenerate, protocol.HandoffGenerateParams{
				Worker: args[0],
			})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var summary protocol.HandoffSummary
			json.Unmarshal(resp.Result, &summary)

			fmt.Printf("Handoff Summary for %s\n", summary.WorkerName)
			fmt.Printf("  Status:  %s\n", summary.Status)
			if summary.JobID != "" {
				fmt.Printf("  Job:     %s\n", summary.JobID[:8])
			}
			fmt.Printf("  Created: %s\n", time.Unix(summary.CreatedAt, 0).Format("2006-01-02 15:04:05"))

			if len(summary.Decisions) > 0 {
				fmt.Println("\nKey Decisions:")
				for _, d := range summary.Decisions {
					fmt.Printf("  - %s\n", d)
				}
			}

			if len(summary.FilesTouched) > 0 {
				fmt.Println("\nFiles Touched:")
				for _, f := range summary.FilesTouched {
					fmt.Printf("  - %s\n", f)
				}
			}

			if len(summary.OpenQuestions) > 0 {
				fmt.Println("\nOpen Questions:")
				for _, q := range summary.OpenQuestions {
					fmt.Printf("  - %s\n", q)
				}
			}

			return nil
		},
	}
}

func workerDetailCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "detail <name>",
		Short: "Show detailed worker information",
		Aliases: []string{"info", "show"},
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodWorkerDetail, map[string]string{
				"name": args[0],
			})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var info protocol.WorkerDetailInfo
			json.Unmarshal(resp.Result, &info)

			fmt.Printf("Worker: %s\n", info.Name)
			fmt.Printf("  ID:            %s\n", info.ID)
			fmt.Printf("  Role:          %s\n", info.Role)
			fmt.Printf("  Status:        %s\n", info.Status)
			if info.CurrentJob != "" {
				fmt.Printf("  Current Job:   %s\n", info.CurrentJob[:8])
			}
			if info.Worktree != "" {
				fmt.Printf("  Worktree:      %s\n", info.Worktree)
			}
			if info.Branch != "" {
				fmt.Printf("  Branch:        %s\n", info.Branch)
			}
			fmt.Printf("  Jobs Completed: %d\n", info.JobsCompleted)
			fmt.Printf("  Jobs Failed:    %d\n", info.JobsFailed)
			if info.TotalCost != "" && info.TotalCost != "$0.00" {
				fmt.Printf("  Total Cost:    %s (%d tokens)\n", info.TotalCost, info.TotalTokens)
			}
			fmt.Printf("  Created:       %s\n", time.Unix(info.CreatedAt, 0).Format("2006-01-02 15:04:05"))

			return nil
		},
	}
}

// Job commands

func jobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Manage jobs",
	}

	cmd.AddCommand(
		jobAddCmd(),
		jobListCmd(),
		jobCancelCmd(),
	)

	return cmd
}

func jobAddCmd() *cobra.Command {
	var worker string
	var priority int

	cmd := &cobra.Command{
		Use:   "add <description>",
		Short: "Add a new job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			params := protocol.JobAddParams{
				Description: args[0],
				Worker:      worker,
				Priority:    priority,
			}

			resp, err := client.Call(protocol.MethodJobAdd, params)
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var info protocol.JobInfo
			json.Unmarshal(resp.Result, &info)

			fmt.Printf("Job created:\n")
			fmt.Printf("  ID:          %s\n", info.ID[:8])
			fmt.Printf("  Description: %s\n", info.Description)
			fmt.Printf("  Status:      %s\n", info.Status)
			fmt.Printf("  Priority:    %d\n", info.Priority)

			return nil
		},
	}

	cmd.Flags().StringVarP(&worker, "worker", "w", "", "Assign to specific worker")
	cmd.Flags().IntVarP(&priority, "priority", "p", 3, "Job priority (1-5)")

	return cmd
}

func jobListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all jobs",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodJobList, nil)
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var jobs []protocol.JobInfo
			json.Unmarshal(resp.Result, &jobs)

			if len(jobs) == 0 {
				fmt.Println("No jobs")
				return nil
			}

			fmt.Printf("%-10s %-12s %-4s %-40s\n", "ID", "STATUS", "PRI", "DESCRIPTION")
			for _, j := range jobs {
				desc := j.Description
				if len(desc) > 38 {
					desc = desc[:38] + ".."
				}
				fmt.Printf("%-10s %-12s %-4d %-40s\n", j.ID[:8], j.Status, j.Priority, desc)
			}

			return nil
		},
	}
}

func jobCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <id>",
		Short: "Cancel a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodJobCancel, map[string]string{"id": args[0]})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			fmt.Printf("Job '%s' cancelled\n", args[0])
			return nil
		},
	}
}

// Review commands

func reviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Manage code reviews",
	}

	cmd.AddCommand(
		reviewStartCmd(),
		reviewStatusCmd(),
		reviewListCmd(),
	)

	return cmd
}

func reviewStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <job-id>",
		Short: "Start a code review for a completed job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodReviewStart, protocol.ReviewStartParams{
				JobID: args[0],
			})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			fmt.Printf("Review started for job %s\n", args[0])
			return nil
		},
	}
}

func reviewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <job-id>",
		Short: "Show status of a code review",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodReviewStatus, protocol.ReviewStatusParams{
				JobID: args[0],
			})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var result protocol.ReviewStatusResult
			json.Unmarshal(resp.Result, &result)

			fmt.Printf("Review Status:\n")
			fmt.Printf("  Job ID:   %s\n", result.JobID)
			fmt.Printf("  Worker:   %s\n", result.WorkerName)
			fmt.Printf("  Phase:    %s\n", result.Phase)
			if result.Decision != "" {
				fmt.Printf("  Decision: %s\n", result.Decision)
			}
			if result.Summary != "" {
				fmt.Printf("  Summary:  %s\n", result.Summary)
			}
			if result.Error != "" {
				fmt.Printf("  Error:    %s\n", result.Error)
			}

			return nil
		},
	}
}

func reviewListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all active reviews",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodReviewList, nil)
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var result protocol.ReviewListResult
			json.Unmarshal(resp.Result, &result)

			if len(result.Reviews) == 0 {
				fmt.Println("No active reviews")
				return nil
			}

			fmt.Printf("%-10s %-15s %-12s %-10s %s\n", "JOB ID", "WORKER", "PHASE", "DECISION", "SUMMARY")
			for _, r := range result.Reviews {
				jobID := r.JobID
				if len(jobID) > 8 {
					jobID = jobID[:8]
				}
				summary := r.Summary
				if len(summary) > 30 {
					summary = summary[:30] + ".."
				}
				decision := r.Decision
				if decision == "" {
					decision = "-"
				}
				fmt.Printf("%-10s %-15s %-12s %-10s %s\n", jobID, r.WorkerName, r.Phase, decision, summary)
			}

			return nil
		},
	}
}

// Operation commands

func operationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "operation",
		Short:   "Manage operations (batch jobs)",
		Aliases: []string{"op"},
	}

	cmd.AddCommand(
		operationCreateCmd(),
		operationStatusCmd(),
		operationListCmd(),
		operationCancelCmd(),
	)

	return cmd
}

func operationCreateCmd() *cobra.Command {
	var jobs []string
	var description string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new operation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			params := protocol.OperationCreateParams{
				Name:        args[0],
				Description: description,
				Jobs:        jobs,
			}

			resp, err := client.Call(protocol.MethodOperationCreate, params)
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var info protocol.OperationInfo
			json.Unmarshal(resp.Result, &info)

			fmt.Printf("Operation created:\n")
			fmt.Printf("  ID:     %s\n", info.ID[:8])
			fmt.Printf("  Name:   %s\n", info.Name)
			fmt.Printf("  Status: %s\n", info.Status)
			fmt.Printf("  Jobs:   %d\n", info.TotalJobs)

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&jobs, "jobs", nil, "Comma-separated job IDs")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Operation description")

	return cmd
}

func operationStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <id>",
		Short: "Show operation status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodOperationStatus, protocol.OperationStatusParams{
				ID: args[0],
			})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var info protocol.OperationInfo
			json.Unmarshal(resp.Result, &info)

			fmt.Printf("Operation: %s\n", info.Name)
			fmt.Printf("  ID:        %s\n", info.ID)
			fmt.Printf("  Status:    %s\n", info.Status)
			fmt.Printf("  Progress:  %d%%\n", info.Progress)
			fmt.Printf("  Jobs:      %d total, %d completed, %d failed\n",
				info.TotalJobs, info.CompletedJobs, info.FailedJobs)
			if info.Description != "" {
				fmt.Printf("  Description: %s\n", info.Description)
			}

			return nil
		},
	}
}

func operationListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all operations",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodOperationList, nil)
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var result protocol.OperationListResult
			json.Unmarshal(resp.Result, &result)

			if len(result.Operations) == 0 {
				fmt.Println("No operations")
				return nil
			}

			fmt.Printf("%-10s %-20s %-12s %-8s %s\n", "ID", "NAME", "STATUS", "PROGRESS", "JOBS")
			for _, op := range result.Operations {
				id := op.ID
				if len(id) > 8 {
					id = id[:8]
				}
				name := op.Name
				if len(name) > 18 {
					name = name[:18] + ".."
				}
				fmt.Printf("%-10s %-20s %-12s %6d%%  %d/%d\n",
					id, name, op.Status, op.Progress, op.CompletedJobs, op.TotalJobs)
			}

			return nil
		},
	}
}

func operationCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <id>",
		Short: "Cancel an operation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodOperationCancel, protocol.OperationCancelParams{
				ID: args[0],
			})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			fmt.Printf("Operation '%s' cancelled\n", args[0])
			return nil
		},
	}
}

// Order commands

func orderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order",
		Short: "Manage standing orders for workers",
	}

	cmd.AddCommand(
		orderSetCmd(),
		orderListCmd(),
		orderClearCmd(),
	)

	return cmd
}

func orderSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <worker> <order>...",
		Short: "Set standing orders for a worker",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodOrderSet, protocol.OrderSetParams{
				Worker: args[0],
				Orders: args[1:],
			})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var result protocol.OrderListResult
			json.Unmarshal(resp.Result, &result)

			fmt.Printf("Standing orders for %s:\n", result.Worker)
			for i, order := range result.Orders {
				fmt.Printf("  %d. %s\n", i+1, order)
			}

			return nil
		},
	}
}

func orderListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list <worker>",
		Short:   "List standing orders for a worker",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodOrderList, protocol.OrderListParams{
				Worker: args[0],
			})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			var result protocol.OrderListResult
			json.Unmarshal(resp.Result, &result)

			if len(result.Orders) == 0 {
				fmt.Printf("No standing orders for %s\n", result.Worker)
				return nil
			}

			fmt.Printf("Standing orders for %s:\n", result.Worker)
			for i, order := range result.Orders {
				fmt.Printf("  %d. %s\n", i+1, order)
			}

			return nil
		},
	}
}

func orderClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear <worker>",
		Short: "Clear all standing orders for a worker",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			resp, err := client.Call(protocol.MethodOrderClear, protocol.OrderClearParams{
				Worker: args[0],
			})
			if err != nil {
				return err
			}

			if resp.Error != nil {
				return fmt.Errorf("%s", resp.Error.Message)
			}

			fmt.Printf("Standing orders cleared for %s\n", args[0])
			return nil
		},
	}
}

// Logs command

func logsCmd() *cobra.Command {
	var workerFilter string
	var follow bool
	var count int

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Stream activity log",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("daemon not running")
			}
			defer client.Close()

			if follow {
				// Subscribe to real-time events
				return streamLogs(client, workerFilter)
			}

			// Read historical logs from ledger
			return showRecentLogs(count, workerFilter)
		},
	}

	cmd.Flags().StringVarP(&workerFilter, "worker", "w", "", "Filter by worker name")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVarP(&count, "count", "n", 50, "Number of recent events to show")

	return cmd
}

func streamLogs(client *daemon.Client, workerFilter string) error {
	// Subscribe to events
	events := []string{"*"}
	if workerFilter != "" {
		events = []string{"worker.started", "worker.stopped", "job.started", "job.completed", "job.failed"}
	}

	if err := client.Subscribe(events); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	fmt.Println("Streaming logs (Ctrl+C to stop)...")

	for {
		event, err := client.ReadEvent()
		if err != nil {
			return err
		}

		// Apply worker filter if specified
		if workerFilter != "" {
			// Check if event data contains worker name
			var data map[string]interface{}
			json.Unmarshal(event.Data, &data)
			if name, ok := data["name"].(string); ok {
				if name != workerFilter {
					continue
				}
			}
		}

		// Format and print event
		ts := event.Timestamp.Format("15:04:05")
		fmt.Printf("[%s] %s", ts, event.Type)

		var data map[string]interface{}
		if json.Unmarshal(event.Data, &data) == nil {
			if name, ok := data["name"]; ok {
				fmt.Printf(" worker=%s", name)
			}
			if id, ok := data["id"]; ok {
				idStr := fmt.Sprint(id)
				if len(idStr) > 8 {
					idStr = idStr[:8]
				}
				fmt.Printf(" id=%s", idStr)
			}
			if errMsg, ok := data["error"]; ok {
				fmt.Printf(" error=%s", errMsg)
			}
		}
		fmt.Println()
	}
}

func showRecentLogs(count int, workerFilter string) error {
	// Read from ledger file
	events, err := readLedgerTail(cfg.LedgerPath(), count)
	if err != nil {
		return fmt.Errorf("failed to read ledger: %w", err)
	}

	for _, event := range events {
		// Apply worker filter if specified
		if workerFilter != "" {
			var data map[string]interface{}
			json.Unmarshal(event.Data, &data)
			if name, ok := data["name"].(string); ok {
				if name != workerFilter {
					continue
				}
			}
		}

		ts := event.Timestamp.Format("2006-01-02 15:04:05")
		fmt.Printf("[%s] %s", ts, event.Type)

		var data map[string]interface{}
		if json.Unmarshal(event.Data, &data) == nil {
			for k, v := range data {
				if k == "id" {
					idStr := fmt.Sprint(v)
					if len(idStr) > 8 {
						idStr = idStr[:8]
					}
					fmt.Printf(" %s=%s", k, idStr)
				} else if k != "" && v != nil && v != "" {
					fmt.Printf(" %s=%v", k, v)
				}
			}
		}
		fmt.Println()
	}

	return nil
}

func readLedgerTail(path string, n int) ([]daemon.LedgerEvent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	lines := splitLines(string(data))
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	var events []daemon.LedgerEvent
	for _, line := range lines {
		if line == "" {
			continue
		}
		var event daemon.LedgerEvent
		if err := json.Unmarshal([]byte(line), &event); err == nil {
			events = append(events, event)
		}
	}

	return events, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// Settings command

func settingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "settings",
		Short:   "View and modify Cosa settings",
		Aliases: []string{"config"},
	}

	cmd.AddCommand(
		settingsListCmd(),
		settingsGetCmd(),
		settingsSetCmd(),
		settingsPathCmd(),
	)

	return cmd
}

func getConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(homeDir, ".cosa", "config.yaml"),
		filepath.Join(homeDir, ".config", "cosa", "config.yaml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// Default path (will be created if needed)
	return filepath.Join(homeDir, ".cosa", "config.yaml")
}

func settingsPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show the config file path",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(getConfigPath())
		},
	}
}

func settingsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all settings with their current values",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Current settings:")
			fmt.Println()

			// Core settings
			fmt.Println("Core:")
			fmt.Printf("  log_level          = %s\n", cfg.LogLevel)
			fmt.Printf("  socket_path        = %s\n", cfg.SocketPath)
			fmt.Printf("  data_dir           = %s\n", cfg.DataDir)
			fmt.Println()

			// Claude settings
			fmt.Println("Claude:")
			fmt.Printf("  claude.binary      = %s\n", cfg.Claude.Binary)
			fmt.Printf("  claude.model       = %s\n", valueOrDefault(cfg.Claude.Model, "(default)"))
			fmt.Printf("  claude.max_turns   = %d\n", cfg.Claude.MaxTurns)
			fmt.Println()

			// Worker settings
			fmt.Println("Workers:")
			fmt.Printf("  workers.max_concurrent = %d\n", cfg.Workers.MaxConcurrent)
			fmt.Printf("  workers.default_role   = %s\n", cfg.Workers.DefaultRole)
			fmt.Println()

			// TUI settings
			fmt.Println("TUI:")
			fmt.Printf("  tui.theme          = %s\n", cfg.TUI.Theme)
			fmt.Printf("  tui.refresh_rate   = %d\n", cfg.TUI.RefreshRate)
			fmt.Println()

			// Notification settings
			fmt.Println("Notifications:")
			fmt.Printf("  notifications.tui_alerts           = %t\n", cfg.Notifications.TUIAlerts)
			fmt.Printf("  notifications.system_notifications = %t\n", cfg.Notifications.SystemNotifications)
			fmt.Printf("  notifications.terminal_bell        = %t\n", cfg.Notifications.TerminalBell)
			fmt.Printf("  notifications.on_job_complete      = %t\n", cfg.Notifications.OnJobComplete)
			fmt.Printf("  notifications.on_job_failed        = %t\n", cfg.Notifications.OnJobFailed)
			fmt.Printf("  notifications.on_worker_stuck      = %t\n", cfg.Notifications.OnWorkerStuck)
			fmt.Println()

			// Model settings
			fmt.Println("Models:")
			fmt.Printf("  models.default     = %s\n", valueOrDefault(cfg.Models.Default, "(claude default)"))
			fmt.Printf("  models.underboss   = %s\n", cfg.Models.Underboss)
			fmt.Printf("  models.consigliere = %s\n", cfg.Models.Consigliere)
			fmt.Printf("  models.capo        = %s\n", cfg.Models.Capo)
			fmt.Printf("  models.soldato     = %s\n", cfg.Models.Soldato)
			fmt.Printf("  models.associate   = %s\n", cfg.Models.Associate)
			fmt.Printf("  models.lookout     = %s\n", cfg.Models.Lookout)
			fmt.Printf("  models.cleaner     = %s\n", cfg.Models.Cleaner)

			return nil
		},
	}
}

func valueOrDefault(value, defaultVal string) string {
	if value == "" {
		return defaultVal
	}
	return value
}

func settingsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a specific setting value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.ToLower(args[0])
			value, err := getSettingValue(key)
			if err != nil {
				return err
			}
			fmt.Println(value)
			return nil
		},
	}
}

func getSettingValue(key string) (string, error) {
	switch key {
	// Core
	case "log_level":
		return cfg.LogLevel, nil
	case "socket_path":
		return cfg.SocketPath, nil
	case "data_dir":
		return cfg.DataDir, nil

	// Claude
	case "claude.binary":
		return cfg.Claude.Binary, nil
	case "claude.model":
		return cfg.Claude.Model, nil
	case "claude.max_turns":
		return strconv.Itoa(cfg.Claude.MaxTurns), nil

	// Workers
	case "workers.max_concurrent":
		return strconv.Itoa(cfg.Workers.MaxConcurrent), nil
	case "workers.default_role":
		return cfg.Workers.DefaultRole, nil

	// TUI
	case "tui.theme":
		return cfg.TUI.Theme, nil
	case "tui.refresh_rate":
		return strconv.Itoa(cfg.TUI.RefreshRate), nil

	// Notifications
	case "notifications.tui_alerts":
		return strconv.FormatBool(cfg.Notifications.TUIAlerts), nil
	case "notifications.system_notifications":
		return strconv.FormatBool(cfg.Notifications.SystemNotifications), nil
	case "notifications.terminal_bell":
		return strconv.FormatBool(cfg.Notifications.TerminalBell), nil
	case "notifications.on_job_complete":
		return strconv.FormatBool(cfg.Notifications.OnJobComplete), nil
	case "notifications.on_job_failed":
		return strconv.FormatBool(cfg.Notifications.OnJobFailed), nil
	case "notifications.on_worker_stuck":
		return strconv.FormatBool(cfg.Notifications.OnWorkerStuck), nil

	// Models
	case "models.default":
		return cfg.Models.Default, nil
	case "models.underboss":
		return cfg.Models.Underboss, nil
	case "models.consigliere":
		return cfg.Models.Consigliere, nil
	case "models.capo":
		return cfg.Models.Capo, nil
	case "models.soldato":
		return cfg.Models.Soldato, nil
	case "models.associate":
		return cfg.Models.Associate, nil
	case "models.lookout":
		return cfg.Models.Lookout, nil
	case "models.cleaner":
		return cfg.Models.Cleaner, nil

	default:
		return "", fmt.Errorf("unknown setting: %s", key)
	}
}

func settingsSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a setting value",
		Long: `Set a configuration value. Changes are saved to the config file.

Examples:
  cosa settings set tui.theme godfather
  cosa settings set workers.max_concurrent 10
  cosa settings set notifications.terminal_bell true
  cosa settings set models.soldato opus

Note: Some settings require restarting the daemon to take effect.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.ToLower(args[0])
			value := args[1]

			if err := setSettingValue(key, value); err != nil {
				return err
			}

			// Save to config file
			configPath := getConfigPath()
			if err := cfg.Save(configPath); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Set %s = %s\n", key, value)
			fmt.Printf("Config saved to %s\n", configPath)

			// Warn about daemon restart if needed
			if needsDaemonRestart(key) {
				fmt.Println("\nNote: Restart the daemon for this change to take effect.")
			}

			return nil
		},
	}
}

func setSettingValue(key, value string) error {
	switch key {
	// Core
	case "log_level":
		validLevels := []string{"debug", "info", "warn", "error"}
		if !contains(validLevels, value) {
			return fmt.Errorf("invalid log_level: %s (must be one of: %s)", value, strings.Join(validLevels, ", "))
		}
		cfg.LogLevel = value

	case "socket_path":
		cfg.SocketPath = value

	case "data_dir":
		cfg.DataDir = value

	// Claude
	case "claude.binary":
		cfg.Claude.Binary = value

	case "claude.model":
		cfg.Claude.Model = value

	case "claude.max_turns":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("invalid max_turns: %s (must be a positive integer)", value)
		}
		cfg.Claude.MaxTurns = n

	// Workers
	case "workers.max_concurrent":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("invalid max_concurrent: %s (must be a positive integer)", value)
		}
		cfg.Workers.MaxConcurrent = n

	case "workers.default_role":
		validRoles := []string{"soldato", "capo", "consigliere", "underboss", "associate", "lookout", "cleaner"}
		if !contains(validRoles, value) {
			return fmt.Errorf("invalid default_role: %s (must be one of: %s)", value, strings.Join(validRoles, ", "))
		}
		cfg.Workers.DefaultRole = value

	// TUI
	case "tui.theme":
		validThemes := []string{"noir", "godfather", "miami", "opencode"}
		if !contains(validThemes, value) {
			return fmt.Errorf("invalid theme: %s (must be one of: %s)", value, strings.Join(validThemes, ", "))
		}
		cfg.TUI.Theme = value

	case "tui.refresh_rate":
		n, err := strconv.Atoi(value)
		if err != nil || n < 10 {
			return fmt.Errorf("invalid refresh_rate: %s (must be at least 10 ms)", value)
		}
		cfg.TUI.RefreshRate = n

	// Notifications
	case "notifications.tui_alerts":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean: %s (use true/false)", value)
		}
		cfg.Notifications.TUIAlerts = b

	case "notifications.system_notifications":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean: %s (use true/false)", value)
		}
		cfg.Notifications.SystemNotifications = b

	case "notifications.terminal_bell":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean: %s (use true/false)", value)
		}
		cfg.Notifications.TerminalBell = b

	case "notifications.on_job_complete":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean: %s (use true/false)", value)
		}
		cfg.Notifications.OnJobComplete = b

	case "notifications.on_job_failed":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean: %s (use true/false)", value)
		}
		cfg.Notifications.OnJobFailed = b

	case "notifications.on_worker_stuck":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean: %s (use true/false)", value)
		}
		cfg.Notifications.OnWorkerStuck = b

	// Models
	case "models.default":
		cfg.Models.Default = value

	case "models.underboss":
		cfg.Models.Underboss = value

	case "models.consigliere":
		cfg.Models.Consigliere = value

	case "models.capo":
		cfg.Models.Capo = value

	case "models.soldato":
		cfg.Models.Soldato = value

	case "models.associate":
		cfg.Models.Associate = value

	case "models.lookout":
		cfg.Models.Lookout = value

	case "models.cleaner":
		cfg.Models.Cleaner = value

	default:
		return fmt.Errorf("unknown setting: %s\n\nRun 'cosa settings list' to see available settings", key)
	}

	return nil
}

func needsDaemonRestart(key string) bool {
	// Settings that require daemon restart
	restartKeys := []string{
		"socket_path",
		"data_dir",
		"log_level",
		"claude.binary",
		"claude.model",
		"claude.max_turns",
		"workers.max_concurrent",
	}
	return contains(restartKeys, key)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// TUI command

func tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "tui",
		Aliases: []string{"t"},
		Short:   "Launch the interactive TUI dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Ensure daemon is running
			if !daemon.IsRunning(cfg.SocketPath) {
				if err := startDaemonBackground(); err != nil {
					return fmt.Errorf("failed to start daemon: %w", err)
				}
			}

			client, err := daemon.Connect(cfg.SocketPath)
			if err != nil {
				return fmt.Errorf("failed to connect to daemon: %w", err)
			}
			defer client.Close()

			return tui.Run(client)
		},
	}
}

// Helper functions

func connectOrStartDaemon() (*daemon.Client, error) {
	if daemon.IsRunning(cfg.SocketPath) {
		return daemon.Connect(cfg.SocketPath)
	}

	// Start daemon
	if err := startDaemonBackground(); err != nil {
		return nil, err
	}

	return daemon.Connect(cfg.SocketPath)
}

func startDaemonBackground() error {
	// Find the cosad binary
	cosad, err := exec.LookPath("cosad")
	if err != nil {
		// Try relative path
		cosad = "./cosad"
		if _, err := os.Stat(cosad); err != nil {
			// Try bin directory
			cosad = "./bin/cosad"
		}
	}

	cmd := exec.Command(cosad)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for daemon to be ready
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if daemon.IsRunning(cfg.SocketPath) {
			fmt.Printf("Cosa daemon started (pid: %d)\n", cmd.Process.Pid)
			return nil
		}
	}

	return fmt.Errorf("daemon failed to start")
}

func runDaemonForeground() error {
	server, err := daemon.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	fmt.Printf("Cosa daemon v%s started (pid: %d)\n", config.Version, os.Getpid())
	fmt.Printf("Listening on %s\n", cfg.SocketPath)
	fmt.Println("Press Ctrl+C to stop")

	server.Wait()
	server.Stop()

	return nil
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

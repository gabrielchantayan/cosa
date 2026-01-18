// Package review implements code review functionality for Cosa.
package review

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"cosa/internal/job"
)

// GateType identifies the type of quality gate.
type GateType string

const (
	GateTest  GateType = "test"
	GateBuild GateType = "build"
)

// GateResult contains the result of running a quality gate.
type GateResult struct {
	Gate     GateType      `json:"gate"`
	Passed   bool          `json:"passed"`
	Output   string        `json:"output"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

// GateRunnerConfig configures the gate runner.
type GateRunnerConfig struct {
	TestCommand  string
	BuildCommand string
	Timeout      time.Duration
}

// GateRunner runs quality gates before code review.
type GateRunner struct {
	testCommand  string
	buildCommand string
	timeout      time.Duration
}

// NewGateRunner creates a new gate runner.
func NewGateRunner(cfg GateRunnerConfig) *GateRunner {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	return &GateRunner{
		testCommand:  cfg.TestCommand,
		buildCommand: cfg.BuildCommand,
		timeout:      timeout,
	}
}

// RunGates runs all configured quality gates for a job.
func (g *GateRunner) RunGates(ctx context.Context, j *job.Job, worktreePath string) ([]GateResult, error) {
	var results []GateResult

	// Run build gate if configured
	if g.buildCommand != "" {
		result := g.runGate(ctx, GateBuild, g.buildCommand, worktreePath)
		results = append(results, result)

		// If build fails, don't run tests
		if !result.Passed {
			return results, nil
		}
	}

	// Run test gate if configured
	if g.testCommand != "" {
		result := g.runGate(ctx, GateTest, g.testCommand, worktreePath)
		results = append(results, result)
	}

	return results, nil
}

// runGate runs a single quality gate command.
func (g *GateRunner) runGate(ctx context.Context, gateType GateType, command, worktreePath string) GateResult {
	start := time.Now()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	// Split command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return GateResult{
			Gate:     gateType,
			Passed:   false,
			Error:    "empty command",
			Duration: time.Since(start),
		}
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = worktreePath

	// Capture output
	output, err := cmd.CombinedOutput()

	result := GateResult{
		Gate:     gateType,
		Output:   string(output),
		Duration: time.Since(start),
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = "command timed out"
		} else {
			result.Error = err.Error()
		}
		result.Passed = false
	} else {
		result.Passed = true
	}

	return result
}

// AllPassed returns true if all gates passed.
func AllPassed(results []GateResult) bool {
	for _, r := range results {
		if !r.Passed {
			return false
		}
	}
	return true
}

// FailedGates returns the list of failed gates.
func FailedGates(results []GateResult) []GateResult {
	var failed []GateResult
	for _, r := range results {
		if !r.Passed {
			failed = append(failed, r)
		}
	}
	return failed
}

// GateResultsSummary returns a human-readable summary of gate results.
func GateResultsSummary(results []GateResult) string {
	if len(results) == 0 {
		return "No gates configured"
	}

	var parts []string
	for _, r := range results {
		status := "✓"
		if !r.Passed {
			status = "✗"
		}
		parts = append(parts, fmt.Sprintf("%s %s (%s)", status, r.Gate, r.Duration.Round(time.Millisecond)))
	}

	return strings.Join(parts, ", ")
}

package review

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"cosa/internal/job"
)

// Decision represents the review decision.
type Decision string

const (
	DecisionApproved Decision = "approved"
	DecisionRejected Decision = "rejected"
)

// ReviewResult contains the result of a code review.
type ReviewResult struct {
	Decision Decision `json:"decision"`
	Summary  string   `json:"summary"`
	Feedback string   `json:"feedback"`
	MustFix  []string `json:"must_fix,omitempty"`
}

// ReviewContext provides context for the code review.
type ReviewContext struct {
	Job         *job.Job     `json:"job"`
	Diff        string       `json:"diff"`
	GateResults []GateResult `json:"gate_results"`
	BaseBranch  string       `json:"base_branch"`
	WorkerName  string       `json:"worker_name"`
}

// ConsigliereConfig configures the Consigliere reviewer.
type ConsigliereConfig struct {
	Binary   string
	Model    string
	MaxTurns int
}

// Consigliere is an AI code reviewer that uses Claude.
type Consigliere struct {
	binary   string
	model    string
	maxTurns int
}

// NewConsigliere creates a new Consigliere reviewer.
func NewConsigliere(cfg ConsigliereConfig) *Consigliere {
	if cfg.Binary == "" {
		cfg.Binary = "claude"
	}
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = 10
	}

	return &Consigliere{
		binary:   cfg.Binary,
		model:    cfg.Model,
		maxTurns: cfg.MaxTurns,
	}
}

// Review performs a code review on the given context.
func (c *Consigliere) Review(ctx context.Context, reviewCtx *ReviewContext) (*ReviewResult, error) {
	prompt := c.buildReviewPrompt(reviewCtx)

	args := []string{
		"--print",
		"--dangerously-skip-permissions",
	}

	if c.model != "" {
		args = append(args, "--model", c.model)
	}

	args = append(args, "--max-turns", fmt.Sprintf("%d", c.maxTurns))
	args = append(args, "-p", prompt)

	cmd := exec.CommandContext(ctx, c.binary, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("claude review failed: %w: %s", err, string(output))
	}

	return c.parseReviewOutput(string(output))
}

// buildReviewPrompt constructs the review prompt for Claude.
func (c *Consigliere) buildReviewPrompt(ctx *ReviewContext) string {
	var sb strings.Builder

	sb.WriteString(`You are a Consigliere (code reviewer) for the Cosa development team.
Your role is to review code changes and provide feedback.

## Instructions
Review the following diff carefully and provide your assessment.
You must respond with a structured decision using the exact format below.

## Response Format (REQUIRED)
You MUST include these sections in your response:

DECISION: [APPROVED or REJECTED]
SUMMARY: [One sentence summary of the changes]
FEEDBACK: [Detailed feedback about the code quality, potential issues, and suggestions]
MUST_FIX: [Comma-separated list of critical issues that must be fixed before approval, or "none" if approved]

## Job Information
`)
	sb.WriteString(fmt.Sprintf("Task: %s\n", ctx.Job.Description))
	sb.WriteString(fmt.Sprintf("Worker: %s\n", ctx.WorkerName))
	sb.WriteString(fmt.Sprintf("Base Branch: %s\n", ctx.BaseBranch))

	if len(ctx.GateResults) > 0 {
		sb.WriteString("\n## Quality Gate Results\n")
		sb.WriteString(GateResultsSummary(ctx.GateResults))
		sb.WriteString("\n")
	}

	sb.WriteString("\n## Diff to Review\n```diff\n")
	// Truncate diff if too long
	diff := ctx.Diff
	if len(diff) > 50000 {
		diff = diff[:50000] + "\n... (diff truncated)"
	}
	sb.WriteString(diff)
	sb.WriteString("\n```\n")

	sb.WriteString(`
## Review Criteria
1. Code correctness - Does it accomplish the task?
2. Code quality - Is it clean, readable, and maintainable?
3. Potential bugs - Are there any obvious issues?
4. Security - Are there any security concerns?
5. Performance - Are there any performance issues?

Provide your structured response now.
`)

	return sb.String()
}

// parseReviewOutput parses Claude's review response.
func (c *Consigliere) parseReviewOutput(output string) (*ReviewResult, error) {
	result := &ReviewResult{
		Decision: DecisionRejected, // Default to rejected if parsing fails
	}

	// Parse DECISION
	decisionRe := regexp.MustCompile(`(?i)DECISION:\s*(APPROVED|REJECTED)`)
	if matches := decisionRe.FindStringSubmatch(output); len(matches) > 1 {
		if strings.EqualFold(matches[1], "APPROVED") {
			result.Decision = DecisionApproved
		} else {
			result.Decision = DecisionRejected
		}
	}

	// Parse SUMMARY
	summaryRe := regexp.MustCompile(`(?i)SUMMARY:\s*(.+?)(?:\n|FEEDBACK:|MUST_FIX:|$)`)
	if matches := summaryRe.FindStringSubmatch(output); len(matches) > 1 {
		result.Summary = strings.TrimSpace(matches[1])
	}

	// Parse FEEDBACK - capture everything between FEEDBACK: and MUST_FIX: (or end)
	feedbackRe := regexp.MustCompile(`(?is)FEEDBACK:\s*(.+?)(?:MUST_FIX:|$)`)
	if matches := feedbackRe.FindStringSubmatch(output); len(matches) > 1 {
		result.Feedback = strings.TrimSpace(matches[1])
	}

	// Parse MUST_FIX
	mustFixRe := regexp.MustCompile(`(?i)MUST_FIX:\s*(.+?)(?:\n\n|$)`)
	if matches := mustFixRe.FindStringSubmatch(output); len(matches) > 1 {
		mustFixStr := strings.TrimSpace(matches[1])
		if !strings.EqualFold(mustFixStr, "none") && mustFixStr != "" {
			// Split by comma or newline
			items := regexp.MustCompile(`[,\n]+`).Split(mustFixStr, -1)
			for _, item := range items {
				item = strings.TrimSpace(item)
				item = strings.TrimPrefix(item, "- ")
				item = strings.TrimPrefix(item, "* ")
				if item != "" && !strings.EqualFold(item, "none") {
					result.MustFix = append(result.MustFix, item)
				}
			}
		}
	}

	// If we couldn't parse a decision, try to infer from content
	if result.Summary == "" && result.Feedback == "" {
		// Fall back to using the entire output as feedback
		result.Feedback = output
		// Try to infer decision from content
		lowerOutput := strings.ToLower(output)
		if strings.Contains(lowerOutput, "looks good") ||
			strings.Contains(lowerOutput, "lgtm") ||
			strings.Contains(lowerOutput, "approve") {
			result.Decision = DecisionApproved
		}
	}

	return result, nil
}

// ReviewResultFromOutput parses review output line by line (alternative parser).
func ReviewResultFromOutput(output string) *ReviewResult {
	result := &ReviewResult{
		Decision: DecisionRejected,
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	var currentSection string
	var feedbackLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(strings.ToUpper(line), "DECISION:") {
			currentSection = "decision"
			value := strings.TrimSpace(strings.TrimPrefix(line, "DECISION:"))
			value = strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(line), "DECISION:"))
			if strings.EqualFold(value, "APPROVED") {
				result.Decision = DecisionApproved
			}
		} else if strings.HasPrefix(strings.ToUpper(line), "SUMMARY:") {
			currentSection = "summary"
			result.Summary = strings.TrimSpace(line[8:])
		} else if strings.HasPrefix(strings.ToUpper(line), "FEEDBACK:") {
			currentSection = "feedback"
			if len(line) > 9 {
				feedbackLines = append(feedbackLines, strings.TrimSpace(line[9:]))
			}
		} else if strings.HasPrefix(strings.ToUpper(line), "MUST_FIX:") {
			currentSection = "must_fix"
			value := strings.TrimSpace(line[9:])
			if value != "" && !strings.EqualFold(value, "none") {
				result.MustFix = append(result.MustFix, value)
			}
		} else if currentSection == "feedback" && line != "" {
			feedbackLines = append(feedbackLines, line)
		} else if currentSection == "must_fix" && line != "" {
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			if !strings.EqualFold(line, "none") {
				result.MustFix = append(result.MustFix, strings.TrimSpace(line))
			}
		}
	}

	result.Feedback = strings.Join(feedbackLines, "\n")
	return result
}

// Package job implements job management for Cosa.
package job

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TemplateType represents a category of job template.
type TemplateType string

const (
	TemplateTypeRefactor TemplateType = "refactor"
	TemplateTypeTest     TemplateType = "test"
	TemplateTypeDocument TemplateType = "document"
	TemplateTypeReview   TemplateType = "review"
	TemplateTypeCustom   TemplateType = "custom"
)

// Template represents a predefined job configuration.
type Template struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        TemplateType      `json:"type"`
	Prompt      string            `json:"prompt"`
	Priority    int               `json:"priority"`
	Variables   []TemplateVar     `json:"variables,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	BuiltIn     bool              `json:"built_in"`
}

// TemplateVar defines a variable placeholder in a template prompt.
type TemplateVar struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// Expand fills in template variables and returns the expanded prompt.
func (t *Template) Expand(vars map[string]string) (string, error) {
	prompt := t.Prompt

	// Check required variables
	for _, v := range t.Variables {
		val, ok := vars[v.Name]
		if !ok || val == "" {
			if v.Required && v.Default == "" {
				return "", fmt.Errorf("required variable %q not provided", v.Name)
			}
			if v.Default != "" {
				val = v.Default
			}
		}
		// Replace {{var}} with value
		placeholder := "{{" + v.Name + "}}"
		prompt = strings.ReplaceAll(prompt, placeholder, val)
	}

	return prompt, nil
}

// CreateJob creates a new job from this template with the given variables.
func (t *Template) CreateJob(vars map[string]string) (*Job, error) {
	description, err := t.Expand(vars)
	if err != nil {
		return nil, err
	}

	job := New(description)
	job.SetPriority(t.Priority)
	return job, nil
}

// TemplateStore manages job templates.
type TemplateStore struct {
	templates map[string]*Template
	path      string // Directory for custom template persistence
	mu        sync.RWMutex
}

// NewTemplateStore creates a new template store with built-in templates.
func NewTemplateStore() *TemplateStore {
	s := &TemplateStore{
		templates: make(map[string]*Template),
	}
	s.loadBuiltIn()
	return s
}

// NewPersistentTemplateStore creates a template store with disk persistence for custom templates.
func NewPersistentTemplateStore(path string) (*TemplateStore, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create templates directory: %w", err)
	}

	s := &TemplateStore{
		templates: make(map[string]*Template),
		path:      path,
	}

	s.loadBuiltIn()

	if err := s.loadCustom(); err != nil {
		return nil, fmt.Errorf("failed to load custom templates: %w", err)
	}

	return s, nil
}

// loadBuiltIn loads all built-in templates.
func (s *TemplateStore) loadBuiltIn() {
	for _, t := range builtInTemplates {
		s.templates[t.ID] = t
	}
}

// loadCustom loads custom templates from disk.
func (s *TemplateStore) loadCustom() error {
	if s.path == "" {
		return nil
	}

	entries, err := os.ReadDir(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(s.path, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var t Template
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}

		// Custom templates cannot override built-in
		if _, exists := s.templates[t.ID]; exists && s.templates[t.ID].BuiltIn {
			continue
		}

		s.templates[t.ID] = &t
	}

	return nil
}

// Get retrieves a template by ID.
func (s *TemplateStore) Get(id string) (*Template, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.templates[id]
	return t, ok
}

// List returns all templates.
func (s *TemplateStore) List() []*Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	templates := make([]*Template, 0, len(s.templates))
	for _, t := range s.templates {
		templates = append(templates, t)
	}
	return templates
}

// ListByType returns templates of a given type.
func (s *TemplateStore) ListByType(templateType TemplateType) []*Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var templates []*Template
	for _, t := range s.templates {
		if t.Type == templateType {
			templates = append(templates, t)
		}
	}
	return templates
}

// ListBuiltIn returns only built-in templates.
func (s *TemplateStore) ListBuiltIn() []*Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var templates []*Template
	for _, t := range s.templates {
		if t.BuiltIn {
			templates = append(templates, t)
		}
	}
	return templates
}

// Add adds a custom template to the store.
func (s *TemplateStore) Add(t *Template) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cannot override built-in templates
	if existing, ok := s.templates[t.ID]; ok && existing.BuiltIn {
		return fmt.Errorf("cannot override built-in template %q", t.ID)
	}

	t.BuiltIn = false
	s.templates[t.ID] = t

	if s.path != "" {
		return s.saveTemplate(t)
	}
	return nil
}

// Remove removes a custom template from the store.
func (s *TemplateStore) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.templates[id]
	if !ok {
		return fmt.Errorf("template %q not found", id)
	}
	if t.BuiltIn {
		return fmt.Errorf("cannot remove built-in template %q", id)
	}

	delete(s.templates, id)
	if s.path != "" {
		os.Remove(s.templateFilePath(id))
	}
	return nil
}

// Persistence helpers

func (s *TemplateStore) templateFilePath(id string) string {
	return filepath.Join(s.path, id+".json")
}

func (s *TemplateStore) saveTemplate(t *Template) error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.templateFilePath(t.ID), data, 0600)
}

// builtInTemplates contains the predefined job templates.
var builtInTemplates = []*Template{
	// Refactor templates
	{
		ID:          "refactor-file",
		Name:        "Refactor File",
		Description: "Refactor a specific file for better code quality",
		Type:        TemplateTypeRefactor,
		Priority:    PriorityNormal,
		BuiltIn:     true,
		Tags:        []string{"refactor", "code-quality"},
		Variables: []TemplateVar{
			{Name: "file", Description: "Path to the file to refactor", Required: true},
			{Name: "focus", Description: "What to focus on (e.g., readability, performance)", Default: "readability and maintainability"},
		},
		Prompt: `Refactor the file {{file}} with a focus on {{focus}}.

Guidelines:
- Improve code structure and organization
- Extract common patterns into reusable functions if appropriate
- Improve naming for clarity
- Remove dead code and unnecessary complexity
- Maintain existing functionality and API contracts
- Add or update comments only where logic is non-obvious

Make commits as you go. When finished, summarize what you changed.`,
	},
	{
		ID:          "refactor-module",
		Name:        "Refactor Module",
		Description: "Refactor an entire module or package",
		Type:        TemplateTypeRefactor,
		Priority:    PriorityNormal,
		BuiltIn:     true,
		Tags:        []string{"refactor", "architecture"},
		Variables: []TemplateVar{
			{Name: "module", Description: "Module/package path to refactor", Required: true},
			{Name: "goal", Description: "Primary goal of the refactor", Default: "improve maintainability"},
		},
		Prompt: `Refactor the module at {{module}} to {{goal}}.

Guidelines:
- Review the module's structure and dependencies
- Identify areas that could benefit from refactoring
- Improve separation of concerns
- Reduce coupling between components
- Ensure public APIs remain stable unless explicitly changing them
- Update tests if interfaces change

Make commits as you go. When finished, summarize what you changed.`,
	},
	{
		ID:          "refactor-function",
		Name:        "Refactor Function",
		Description: "Refactor a specific function for clarity or performance",
		Type:        TemplateTypeRefactor,
		Priority:    PriorityNormal,
		BuiltIn:     true,
		Tags:        []string{"refactor", "function"},
		Variables: []TemplateVar{
			{Name: "function", Description: "Function name or signature", Required: true},
			{Name: "file", Description: "File containing the function", Required: true},
			{Name: "reason", Description: "Why refactoring is needed", Default: "improve readability"},
		},
		Prompt: `Refactor the function {{function}} in {{file}} to {{reason}}.

Guidelines:
- Preserve the function's behavior and signature unless explicitly changing the API
- Break down complex logic into smaller, well-named helper functions if needed
- Improve variable naming
- Simplify control flow
- Add comments only for non-obvious logic

Make commits as you go. When finished, summarize what you changed.`,
	},

	// Test templates
	{
		ID:          "test-unit",
		Name:        "Write Unit Tests",
		Description: "Write unit tests for a specific file or function",
		Type:        TemplateTypeTest,
		Priority:    PriorityNormal,
		BuiltIn:     true,
		Tags:        []string{"test", "unit"},
		Variables: []TemplateVar{
			{Name: "target", Description: "File or function to test", Required: true},
			{Name: "coverage", Description: "Coverage goal or focus areas", Default: "core functionality and edge cases"},
		},
		Prompt: `Write unit tests for {{target}} covering {{coverage}}.

Guidelines:
- Follow existing test patterns in the codebase
- Test both happy path and error cases
- Include edge cases (empty inputs, boundary values, etc.)
- Use descriptive test names that explain what is being tested
- Mock external dependencies appropriately
- Aim for tests that are fast, isolated, and deterministic

Make commits as you go. When finished, summarize what tests you added and what they cover.`,
	},
	{
		ID:          "test-integration",
		Name:        "Write Integration Tests",
		Description: "Write integration tests for a component or feature",
		Type:        TemplateTypeTest,
		Priority:    PriorityNormal,
		BuiltIn:     true,
		Tags:        []string{"test", "integration"},
		Variables: []TemplateVar{
			{Name: "feature", Description: "Feature or component to test", Required: true},
			{Name: "scope", Description: "Scope of integration (e.g., API, database)", Default: "full feature flow"},
		},
		Prompt: `Write integration tests for {{feature}} covering {{scope}}.

Guidelines:
- Test realistic user flows and scenarios
- Set up and tear down test fixtures properly
- Test error handling and recovery
- Verify interactions between components work correctly
- Keep tests independent and idempotent

Make commits as you go. When finished, summarize what tests you added and what integration points they verify.`,
	},
	{
		ID:          "test-fix",
		Name:        "Fix Failing Tests",
		Description: "Investigate and fix failing tests",
		Type:        TemplateTypeTest,
		Priority:    PriorityHigh,
		BuiltIn:     true,
		Tags:        []string{"test", "fix", "debug"},
		Variables: []TemplateVar{
			{Name: "test", Description: "Test name or file with failures", Required: true},
			{Name: "context", Description: "Additional context about the failures", Default: ""},
		},
		Prompt: `Investigate and fix the failing tests in {{test}}. {{context}}

Guidelines:
- Run the tests first to understand the failures
- Determine if the test or the implementation is wrong
- If the test is wrong, fix the test
- If the implementation is wrong, fix the implementation
- Ensure all related tests pass after your changes
- Do not skip or disable tests unless absolutely necessary

Make commits as you go. When finished, summarize what you found and how you fixed it.`,
	},

	// Document templates
	{
		ID:          "document-file",
		Name:        "Document File",
		Description: "Add or improve documentation for a file",
		Type:        TemplateTypeDocument,
		Priority:    PriorityNormal,
		BuiltIn:     true,
		Tags:        []string{"documentation", "file"},
		Variables: []TemplateVar{
			{Name: "file", Description: "File to document", Required: true},
			{Name: "style", Description: "Documentation style to use", Default: "existing project conventions"},
		},
		Prompt: `Add or improve documentation for {{file}} following {{style}}.

Guidelines:
- Add a file-level comment explaining the purpose and contents
- Document public functions/methods with their purpose, parameters, and return values
- Document complex types and interfaces
- Keep documentation concise and accurate
- Focus on the "why" rather than the "what" where possible
- Use examples where they add clarity

Make commits as you go. When finished, summarize what documentation you added.`,
	},
	{
		ID:          "document-api",
		Name:        "Document API",
		Description: "Document an API endpoint or interface",
		Type:        TemplateTypeDocument,
		Priority:    PriorityNormal,
		BuiltIn:     true,
		Tags:        []string{"documentation", "api"},
		Variables: []TemplateVar{
			{Name: "api", Description: "API endpoint or interface to document", Required: true},
			{Name: "format", Description: "Documentation format", Default: "inline code comments"},
		},
		Prompt: `Document the API {{api}} using {{format}}.

Guidelines:
- Document all endpoints with their HTTP method and path
- Describe request parameters and body schema
- Describe response format and status codes
- Include example requests and responses
- Document authentication requirements
- Note any rate limits or special considerations

Make commits as you go. When finished, summarize what documentation you added.`,
	},
	{
		ID:          "document-architecture",
		Name:        "Document Architecture",
		Description: "Document system or module architecture",
		Type:        TemplateTypeDocument,
		Priority:    PriorityNormal,
		BuiltIn:     true,
		Tags:        []string{"documentation", "architecture"},
		Variables: []TemplateVar{
			{Name: "component", Description: "Component or system to document", Required: true},
			{Name: "audience", Description: "Target audience", Default: "developers new to the codebase"},
		},
		Prompt: `Document the architecture of {{component}} for {{audience}}.

Guidelines:
- Explain the high-level design and component relationships
- Describe the data flow and key interactions
- Document important design decisions and trade-offs
- Include diagrams if they would help (as text/ASCII art)
- Reference relevant code locations
- Explain how to extend or modify the architecture

Make commits as you go. When finished, summarize what documentation you created.`,
	},

	// Review templates
	{
		ID:          "review-code",
		Name:        "Code Review",
		Description: "Review code for quality, bugs, and improvements",
		Type:        TemplateTypeReview,
		Priority:    PriorityHigh,
		BuiltIn:     true,
		Tags:        []string{"review", "code-quality"},
		Variables: []TemplateVar{
			{Name: "target", Description: "File, PR, or branch to review", Required: true},
			{Name: "focus", Description: "Areas to focus on", Default: "correctness, performance, and maintainability"},
		},
		Prompt: `Review {{target}} focusing on {{focus}}.

Provide feedback on:
- Correctness: Are there bugs or logic errors?
- Performance: Are there performance concerns?
- Security: Are there security vulnerabilities?
- Maintainability: Is the code clear and well-structured?
- Testing: Is the code adequately tested?
- Style: Does it follow project conventions?

For each issue found, provide:
1. Location (file and line if applicable)
2. Description of the issue
3. Suggested fix or improvement

Summarize your findings with a list of issues by severity (critical, major, minor, suggestion).`,
	},
	{
		ID:          "review-security",
		Name:        "Security Review",
		Description: "Review code for security vulnerabilities",
		Type:        TemplateTypeReview,
		Priority:    PriorityCritical,
		BuiltIn:     true,
		Tags:        []string{"review", "security"},
		Variables: []TemplateVar{
			{Name: "target", Description: "File, module, or feature to review", Required: true},
			{Name: "threat_model", Description: "Specific threats to consider", Default: "OWASP Top 10"},
		},
		Prompt: `Perform a security review of {{target}} considering {{threat_model}}.

Check for:
- Injection vulnerabilities (SQL, command, XSS, etc.)
- Authentication and authorization issues
- Sensitive data exposure
- Security misconfiguration
- Insufficient input validation
- Improper error handling that leaks information
- Insecure dependencies
- Cryptographic weaknesses

For each finding, provide:
1. Vulnerability type and severity
2. Location in code
3. Potential impact
4. Recommended remediation

Summarize with a security assessment and prioritized list of fixes needed.`,
	},
	{
		ID:          "review-performance",
		Name:        "Performance Review",
		Description: "Review code for performance issues",
		Type:        TemplateTypeReview,
		Priority:    PriorityHigh,
		BuiltIn:     true,
		Tags:        []string{"review", "performance"},
		Variables: []TemplateVar{
			{Name: "target", Description: "File or component to review", Required: true},
			{Name: "context", Description: "Performance context (e.g., high traffic, large data)", Default: "general performance"},
		},
		Prompt: `Review {{target}} for performance issues in the context of {{context}}.

Analyze:
- Algorithm complexity (time and space)
- Database query efficiency
- Memory usage and potential leaks
- I/O operations and blocking
- Caching opportunities
- Unnecessary computations or allocations
- Concurrency issues

For each issue found, provide:
1. Description of the performance concern
2. Location in code
3. Estimated impact
4. Suggested optimization

Summarize with prioritized recommendations for performance improvements.`,
	},
}

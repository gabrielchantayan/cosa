// Package config handles Cosa configuration loading and management.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Version is the current Cosa version.
const Version = "0.1.0"

// Config represents the Cosa configuration.
type Config struct {
	// SocketPath is the Unix socket path for daemon communication.
	SocketPath string `yaml:"socket_path"`

	// DataDir is the directory for Cosa data (ledger, state, etc.).
	DataDir string `yaml:"data_dir"`

	// LogLevel controls logging verbosity (debug, info, warn, error).
	LogLevel string `yaml:"log_level"`

	// Claude contains Claude Code CLI configuration.
	Claude ClaudeConfig `yaml:"claude"`

	// Workers contains worker defaults.
	Workers WorkerConfig `yaml:"workers"`

	// Git contains git-related configuration.
	Git GitConfig `yaml:"git"`

	// TUI contains TUI configuration.
	TUI TUIConfig `yaml:"tui"`

	// Notifications contains notification settings.
	Notifications NotificationConfig `yaml:"notifications"`

	// Models contains per-role model configuration.
	Models ModelConfig `yaml:"models"`
}

// ClaudeConfig contains Claude Code CLI settings.
type ClaudeConfig struct {
	// Path to the claude CLI binary.
	Binary string `yaml:"binary"`

	// Model to use (optional, uses claude default if empty).
	Model string `yaml:"model"`

	// MaxTurns limits the number of turns per session.
	MaxTurns int `yaml:"max_turns"`

	// ChatTimeout is the timeout in seconds for chat responses (default: 120).
	ChatTimeout int `yaml:"chat_timeout"`
}

// WorkerConfig contains worker defaults.
type WorkerConfig struct {
	// MaxConcurrent is the maximum number of concurrent workers.
	MaxConcurrent int `yaml:"max_concurrent"`

	// DefaultRole for new workers.
	DefaultRole string `yaml:"default_role"`
}

// GitConfig contains git-related configuration.
type GitConfig struct {
	// DefaultMergeBranch is the default branch where workers merge their work.
	// This serves as a global default when a territory doesn't have a DevBranch configured.
	// Common values: main, master, staging, dev, develop
	DefaultMergeBranch string `yaml:"default_merge_branch"`
}

// TUIConfig contains TUI settings.
type TUIConfig struct {
	// Theme name (noir, godfather, miami, opencode).
	Theme string `yaml:"theme"`

	// RefreshRate in milliseconds for activity updates.
	RefreshRate int `yaml:"refresh_rate"`
}

// NotificationConfig contains notification settings.
type NotificationConfig struct {
	// TUIAlerts enables in-TUI alerts.
	TUIAlerts bool `yaml:"tui_alerts"`

	// SystemNotifications enables macOS system notifications.
	SystemNotifications bool `yaml:"system_notifications"`

	// TerminalBell enables terminal bell on events.
	TerminalBell bool `yaml:"terminal_bell"`

	// OnJobComplete enables notifications when jobs complete.
	OnJobComplete bool `yaml:"on_job_complete"`

	// OnJobFailed enables notifications when jobs fail.
	OnJobFailed bool `yaml:"on_job_failed"`

	// OnWorkerStuck enables notifications when workers are stuck.
	OnWorkerStuck bool `yaml:"on_worker_stuck"`
}

// ModelConfig contains per-role model configuration.
type ModelConfig struct {
	// Default model for unspecified roles.
	Default string `yaml:"default"`

	// Per-role overrides
	Underboss   string `yaml:"underboss"`
	Consigliere string `yaml:"consigliere"`
	Capo        string `yaml:"capo"`
	Soldato     string `yaml:"soldato"`
	Associate   string `yaml:"associate"`
	Lookout     string `yaml:"lookout"`
	Cleaner     string `yaml:"cleaner"`
}

// ModelForRole returns the configured model for a given role.
func (m *ModelConfig) ModelForRole(role string) string {
	switch role {
	case "underboss":
		if m.Underboss != "" {
			return m.Underboss
		}
	case "consigliere":
		if m.Consigliere != "" {
			return m.Consigliere
		}
	case "capo":
		if m.Capo != "" {
			return m.Capo
		}
	case "soldato":
		if m.Soldato != "" {
			return m.Soldato
		}
	case "associate":
		if m.Associate != "" {
			return m.Associate
		}
	case "lookout":
		if m.Lookout != "" {
			return m.Lookout
		}
	case "cleaner":
		if m.Cleaner != "" {
			return m.Cleaner
		}
	}
	return m.Default
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".cosa")

	return &Config{
		SocketPath: filepath.Join(dataDir, "cosa.sock"),
		DataDir:    dataDir,
		LogLevel:   "info",
		Claude: ClaudeConfig{
			Binary:      "claude",
			MaxTurns:    100,
			ChatTimeout: 120,
		},
		Workers: WorkerConfig{
			MaxConcurrent: 5,
			DefaultRole:   "soldato",
		},
		Git: GitConfig{
			DefaultMergeBranch: "", // Empty means use repository's default branch
		},
		TUI: TUIConfig{
			Theme:       "noir",
			RefreshRate: 100,
		},
		Notifications: NotificationConfig{
			TUIAlerts:           true,
			SystemNotifications: true,
			TerminalBell:        false,
			OnJobComplete:       true,
			OnJobFailed:         true,
			OnWorkerStuck:       true,
		},
		Models: ModelConfig{
			Default:     "", // Use claude default
			Underboss:   "opus",
			Consigliere: "opus",
			Capo:        "opus",
			Soldato:     "sonnet",
			Associate:   "sonnet",
			Lookout:     "haiku",
			Cleaner:     "haiku",
		},
	}
}

// Load reads configuration from file, merging with defaults.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		// Try default locations
		homeDir, _ := os.UserHomeDir()
		candidates := []string{
			filepath.Join(homeDir, ".cosa", "config.yaml"),
			filepath.Join(homeDir, ".config", "cosa", "config.yaml"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}

	if path == "" {
		return cfg, nil // No config file, use defaults
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes configuration to file.
func (c *Config) Save(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// EnsureDataDir creates the data directory if it doesn't exist.
func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0755)
}

// LedgerPath returns the path to the event ledger.
func (c *Config) LedgerPath() string {
	return filepath.Join(c.DataDir, "events.jsonl")
}

// StatePath returns the path to the state file.
func (c *Config) StatePath() string {
	return filepath.Join(c.DataDir, "state.json")
}

// PIDPath returns the path to the daemon PID file.
func (c *Config) PIDPath() string {
	return filepath.Join(c.DataDir, "cosad.pid")
}

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	// Check socket path contains expected directory
	if cfg.SocketPath == "" {
		t.Error("expected non-empty SocketPath")
	}

	// Check data dir is set
	if cfg.DataDir == "" {
		t.Error("expected non-empty DataDir")
	}

	// Check defaults
	if cfg.LogLevel != "info" {
		t.Errorf("expected log level 'info', got '%s'", cfg.LogLevel)
	}

	// Check Claude defaults
	if cfg.Claude.Binary != "claude" {
		t.Errorf("expected claude binary 'claude', got '%s'", cfg.Claude.Binary)
	}
	if cfg.Claude.MaxTurns != 100 {
		t.Errorf("expected max turns 100, got %d", cfg.Claude.MaxTurns)
	}

	// Check worker defaults
	if cfg.Workers.MaxConcurrent != 5 {
		t.Errorf("expected max concurrent 5, got %d", cfg.Workers.MaxConcurrent)
	}
	if cfg.Workers.DefaultRole != "soldato" {
		t.Errorf("expected default role 'soldato', got '%s'", cfg.Workers.DefaultRole)
	}

	// Check TUI defaults
	if cfg.TUI.Theme != "noir" {
		t.Errorf("expected theme 'noir', got '%s'", cfg.TUI.Theme)
	}
	if cfg.TUI.RefreshRate != 100 {
		t.Errorf("expected refresh rate 100, got %d", cfg.TUI.RefreshRate)
	}

	// Check notification defaults
	if !cfg.Notifications.TUIAlerts {
		t.Error("expected TUIAlerts to be true")
	}
	if !cfg.Notifications.SystemNotifications {
		t.Error("expected SystemNotifications to be true")
	}
	if cfg.Notifications.TerminalBell {
		t.Error("expected TerminalBell to be false")
	}
}

func TestLoad_NoConfigFile(t *testing.T) {
	// Use a path that definitely doesn't exist
	cfg, err := Load("/nonexistent/path/to/config.yaml")
	if err != nil {
		t.Errorf("expected no error for nonexistent config, got: %v", err)
	}
	if cfg == nil {
		t.Error("expected default config to be returned")
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Error("expected config to be returned")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
socket_path: /custom/path/cosa.sock
data_dir: /custom/data
log_level: debug
claude:
  binary: /usr/local/bin/claude
  model: claude-3-opus
  max_turns: 50
workers:
  max_concurrent: 10
  default_role: capo
tui:
  theme: godfather
  refresh_rate: 200
notifications:
  tui_alerts: false
  system_notifications: false
  terminal_bell: true
models:
  default: opus
  soldato: haiku
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.SocketPath != "/custom/path/cosa.sock" {
		t.Errorf("expected socket path '/custom/path/cosa.sock', got '%s'", cfg.SocketPath)
	}
	if cfg.DataDir != "/custom/data" {
		t.Errorf("expected data dir '/custom/data', got '%s'", cfg.DataDir)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level 'debug', got '%s'", cfg.LogLevel)
	}
	if cfg.Claude.Binary != "/usr/local/bin/claude" {
		t.Errorf("expected claude binary '/usr/local/bin/claude', got '%s'", cfg.Claude.Binary)
	}
	if cfg.Claude.Model != "claude-3-opus" {
		t.Errorf("expected claude model 'claude-3-opus', got '%s'", cfg.Claude.Model)
	}
	if cfg.Claude.MaxTurns != 50 {
		t.Errorf("expected max turns 50, got %d", cfg.Claude.MaxTurns)
	}
	if cfg.Workers.MaxConcurrent != 10 {
		t.Errorf("expected max concurrent 10, got %d", cfg.Workers.MaxConcurrent)
	}
	if cfg.Workers.DefaultRole != "capo" {
		t.Errorf("expected default role 'capo', got '%s'", cfg.Workers.DefaultRole)
	}
	if cfg.TUI.Theme != "godfather" {
		t.Errorf("expected theme 'godfather', got '%s'", cfg.TUI.Theme)
	}
	if cfg.TUI.RefreshRate != 200 {
		t.Errorf("expected refresh rate 200, got %d", cfg.TUI.RefreshRate)
	}
	if cfg.Notifications.TUIAlerts {
		t.Error("expected TUIAlerts to be false")
	}
	if cfg.Notifications.SystemNotifications {
		t.Error("expected SystemNotifications to be false")
	}
	if !cfg.Notifications.TerminalBell {
		t.Error("expected TerminalBell to be true")
	}
	if cfg.Models.Default != "opus" {
		t.Errorf("expected default model 'opus', got '%s'", cfg.Models.Default)
	}
	if cfg.Models.Soldato != "haiku" {
		t.Errorf("expected soldato model 'haiku', got '%s'", cfg.Models.Soldato)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configPath, []byte("invalid: yaml: content: [[["), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.yaml")

	cfg := DefaultConfig()
	cfg.LogLevel = "debug"
	cfg.TUI.Theme = "miami"

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("expected config file to be created")
	}

	// Load and verify
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if loaded.LogLevel != "debug" {
		t.Errorf("expected log level 'debug', got '%s'", loaded.LogLevel)
	}
	if loaded.TUI.Theme != "miami" {
		t.Errorf("expected theme 'miami', got '%s'", loaded.TUI.Theme)
	}
}

func TestEnsureDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "cosa", "data")

	cfg := &Config{DataDir: dataDir}

	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("failed to ensure data dir: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(dataDir)
	if os.IsNotExist(err) {
		t.Error("expected data dir to be created")
	}
	if !info.IsDir() {
		t.Error("expected data dir to be a directory")
	}

	// Should be idempotent
	if err := cfg.EnsureDataDir(); err != nil {
		t.Errorf("EnsureDataDir should be idempotent: %v", err)
	}
}

func TestLedgerPath(t *testing.T) {
	cfg := &Config{DataDir: "/path/to/data"}

	path := cfg.LedgerPath()
	expected := "/path/to/data/events.jsonl"

	if path != expected {
		t.Errorf("expected ledger path '%s', got '%s'", expected, path)
	}
}

func TestStatePath(t *testing.T) {
	cfg := &Config{DataDir: "/path/to/data"}

	path := cfg.StatePath()
	expected := "/path/to/data/state.json"

	if path != expected {
		t.Errorf("expected state path '%s', got '%s'", expected, path)
	}
}

func TestPIDPath(t *testing.T) {
	cfg := &Config{DataDir: "/path/to/data"}

	path := cfg.PIDPath()
	expected := "/path/to/data/cosad.pid"

	if path != expected {
		t.Errorf("expected PID path '%s', got '%s'", expected, path)
	}
}

func TestModelForRole(t *testing.T) {
	m := &ModelConfig{
		Default:     "default-model",
		Underboss:   "underboss-model",
		Consigliere: "consigliere-model",
		Capo:        "capo-model",
		Soldato:     "soldato-model",
		Associate:   "associate-model",
		Lookout:     "lookout-model",
		Cleaner:     "cleaner-model",
	}

	tests := []struct {
		role     string
		expected string
	}{
		{"underboss", "underboss-model"},
		{"consigliere", "consigliere-model"},
		{"capo", "capo-model"},
		{"soldato", "soldato-model"},
		{"associate", "associate-model"},
		{"lookout", "lookout-model"},
		{"cleaner", "cleaner-model"},
		{"unknown", "default-model"},
		{"", "default-model"},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			result := m.ModelForRole(tt.role)
			if result != tt.expected {
				t.Errorf("ModelForRole(%q) = %q, want %q", tt.role, result, tt.expected)
			}
		})
	}
}

func TestModelForRole_EmptyRoleConfig(t *testing.T) {
	m := &ModelConfig{
		Default: "default-model",
		// All role-specific configs empty
	}

	// All roles should fall back to default
	roles := []string{"underboss", "consigliere", "capo", "soldato", "associate", "lookout", "cleaner"}

	for _, role := range roles {
		result := m.ModelForRole(role)
		if result != "default-model" {
			t.Errorf("ModelForRole(%q) should return default when role config is empty, got %q", role, result)
		}
	}
}

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestConfigStructures(t *testing.T) {
	// Test that all config structs can be instantiated
	cfg := Config{
		SocketPath: "/test.sock",
		DataDir:    "/test/data",
		LogLevel:   "debug",
		Claude: ClaudeConfig{
			Binary:   "claude",
			Model:    "opus",
			MaxTurns: 100,
		},
		Workers: WorkerConfig{
			MaxConcurrent: 5,
			DefaultRole:   "soldato",
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
			Default: "sonnet",
		},
	}

	if cfg.SocketPath != "/test.sock" {
		t.Error("struct field assignment failed")
	}
}

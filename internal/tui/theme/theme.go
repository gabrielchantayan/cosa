// Package theme provides color themes for the TUI.
package theme

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

// Theme defines the color palette for the TUI.
type Theme struct {
	Name string

	// Primary colors
	Primary      lipgloss.Color
	Secondary    lipgloss.Color
	Accent       lipgloss.Color

	// Background colors
	Background   lipgloss.Color
	Surface      lipgloss.Color
	SurfaceLight lipgloss.Color

	// Text colors
	Text         lipgloss.Color
	TextMuted    lipgloss.Color
	TextDim      lipgloss.Color

	// Status colors
	Success      lipgloss.Color
	Warning      lipgloss.Color
	Error        lipgloss.Color
	Info         lipgloss.Color

	// Role colors
	RoleDon         lipgloss.Color
	RoleConsigliere lipgloss.Color
	RoleCapo        lipgloss.Color
	RoleSoldato     lipgloss.Color

	// Border
	Border       lipgloss.Color
	BorderActive lipgloss.Color
}

// Current is the currently active theme.
var Current = Noir

// Noir is the default dark theme - inspired by film noir.
var Noir = Theme{
	Name: "noir",

	Primary:      lipgloss.Color("#B8860B"), // Dark goldenrod
	Secondary:    lipgloss.Color("#8B4513"), // Saddle brown
	Accent:       lipgloss.Color("#CD853F"), // Peru

	Background:   lipgloss.Color("#0D0D0D"),
	Surface:      lipgloss.Color("#1A1A1A"),
	SurfaceLight: lipgloss.Color("#262626"),

	Text:         lipgloss.Color("#E5E5E5"),
	TextMuted:    lipgloss.Color("#A0A0A0"),
	TextDim:      lipgloss.Color("#666666"),

	Success:      lipgloss.Color("#2E8B57"), // Sea green
	Warning:      lipgloss.Color("#DAA520"), // Goldenrod
	Error:        lipgloss.Color("#8B0000"), // Dark red
	Info:         lipgloss.Color("#4682B4"), // Steel blue

	RoleDon:         lipgloss.Color("#FFD700"), // Gold
	RoleConsigliere: lipgloss.Color("#C0C0C0"), // Silver
	RoleCapo:        lipgloss.Color("#CD7F32"), // Bronze
	RoleSoldato:     lipgloss.Color("#808080"), // Gray

	Border:       lipgloss.Color("#333333"),
	BorderActive: lipgloss.Color("#B8860B"),
}

// Godfather is inspired by The Godfather films.
var Godfather = Theme{
	Name: "godfather",

	Primary:      lipgloss.Color("#8B0000"), // Dark red
	Secondary:    lipgloss.Color("#2F4F4F"), // Dark slate gray
	Accent:       lipgloss.Color("#FFD700"), // Gold

	Background:   lipgloss.Color("#0A0A0A"),
	Surface:      lipgloss.Color("#1C1C1C"),
	SurfaceLight: lipgloss.Color("#2D2D2D"),

	Text:         lipgloss.Color("#F5F5DC"), // Beige
	TextMuted:    lipgloss.Color("#A9A9A9"),
	TextDim:      lipgloss.Color("#696969"),

	Success:      lipgloss.Color("#228B22"),
	Warning:      lipgloss.Color("#B8860B"),
	Error:        lipgloss.Color("#DC143C"),
	Info:         lipgloss.Color("#4169E1"),

	RoleDon:         lipgloss.Color("#FFD700"),
	RoleConsigliere: lipgloss.Color("#E6E6FA"),
	RoleCapo:        lipgloss.Color("#8B0000"),
	RoleSoldato:     lipgloss.Color("#2F4F4F"),

	Border:       lipgloss.Color("#3D3D3D"),
	BorderActive: lipgloss.Color("#8B0000"),
}

// Miami is inspired by 80s Miami Vice aesthetic.
var Miami = Theme{
	Name: "miami",

	Primary:      lipgloss.Color("#FF1493"), // Deep pink
	Secondary:    lipgloss.Color("#00CED1"), // Dark turquoise
	Accent:       lipgloss.Color("#FFD700"), // Gold

	Background:   lipgloss.Color("#0D0D1A"),
	Surface:      lipgloss.Color("#1A1A2E"),
	SurfaceLight: lipgloss.Color("#2D2D44"),

	Text:         lipgloss.Color("#FFFFFF"),
	TextMuted:    lipgloss.Color("#B0B0B0"),
	TextDim:      lipgloss.Color("#707070"),

	Success:      lipgloss.Color("#00FF7F"),
	Warning:      lipgloss.Color("#FFD700"),
	Error:        lipgloss.Color("#FF1493"),
	Info:         lipgloss.Color("#00CED1"),

	RoleDon:         lipgloss.Color("#FFD700"),
	RoleConsigliere: lipgloss.Color("#FF1493"),
	RoleCapo:        lipgloss.Color("#00CED1"),
	RoleSoldato:     lipgloss.Color("#9370DB"),

	Border:       lipgloss.Color("#3D3D5C"),
	BorderActive: lipgloss.Color("#FF1493"),
}

// Themes is a map of available themes.
var Themes = map[string]Theme{
	"noir":      Noir,
	"godfather": Godfather,
	"miami":     Miami,
}

// SetTheme sets the current theme by name.
func SetTheme(name string) bool {
	if theme, ok := Themes[name]; ok {
		Current = theme
		return true
	}
	return false
}

// YAMLTheme represents a theme in YAML format.
type YAMLTheme struct {
	Name string `yaml:"name"`

	Primary      string `yaml:"primary"`
	Secondary    string `yaml:"secondary"`
	Accent       string `yaml:"accent"`

	Background   string `yaml:"background"`
	Surface      string `yaml:"surface"`
	SurfaceLight string `yaml:"surface_light"`

	Text         string `yaml:"text"`
	TextMuted    string `yaml:"text_muted"`
	TextDim      string `yaml:"text_dim"`

	Success      string `yaml:"success"`
	Warning      string `yaml:"warning"`
	Error        string `yaml:"error"`
	Info         string `yaml:"info"`

	RoleDon         string `yaml:"role_don"`
	RoleConsigliere string `yaml:"role_consigliere"`
	RoleCapo        string `yaml:"role_capo"`
	RoleSoldato     string `yaml:"role_soldato"`

	Border       string `yaml:"border"`
	BorderActive string `yaml:"border_active"`
}

// LoadThemeFromFile loads a theme from a YAML file.
func LoadThemeFromFile(path string) (*Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var yt YAMLTheme
	if err := yaml.Unmarshal(data, &yt); err != nil {
		return nil, err
	}

	theme := &Theme{
		Name:         yt.Name,
		Primary:      colorOrDefault(yt.Primary, Noir.Primary),
		Secondary:    colorOrDefault(yt.Secondary, Noir.Secondary),
		Accent:       colorOrDefault(yt.Accent, Noir.Accent),
		Background:   colorOrDefault(yt.Background, Noir.Background),
		Surface:      colorOrDefault(yt.Surface, Noir.Surface),
		SurfaceLight: colorOrDefault(yt.SurfaceLight, Noir.SurfaceLight),
		Text:         colorOrDefault(yt.Text, Noir.Text),
		TextMuted:    colorOrDefault(yt.TextMuted, Noir.TextMuted),
		TextDim:      colorOrDefault(yt.TextDim, Noir.TextDim),
		Success:      colorOrDefault(yt.Success, Noir.Success),
		Warning:      colorOrDefault(yt.Warning, Noir.Warning),
		Error:        colorOrDefault(yt.Error, Noir.Error),
		Info:         colorOrDefault(yt.Info, Noir.Info),
		RoleDon:         colorOrDefault(yt.RoleDon, Noir.RoleDon),
		RoleConsigliere: colorOrDefault(yt.RoleConsigliere, Noir.RoleConsigliere),
		RoleCapo:        colorOrDefault(yt.RoleCapo, Noir.RoleCapo),
		RoleSoldato:     colorOrDefault(yt.RoleSoldato, Noir.RoleSoldato),
		Border:       colorOrDefault(yt.Border, Noir.Border),
		BorderActive: colorOrDefault(yt.BorderActive, Noir.BorderActive),
	}

	return theme, nil
}

func colorOrDefault(hex string, fallback lipgloss.Color) lipgloss.Color {
	if hex == "" {
		return fallback
	}
	return lipgloss.Color(hex)
}

// LoadCustomThemes loads all themes from a directory.
func LoadCustomThemes(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, that's fine
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, name)
		theme, err := LoadThemeFromFile(path)
		if err != nil {
			continue // Skip invalid themes
		}

		if theme.Name == "" {
			// Use filename without extension as theme name
			theme.Name = name[:len(name)-len(ext)]
		}

		Themes[theme.Name] = *theme
	}

	return nil
}

// LoadUserThemes loads themes from the user's config directory.
func LoadUserThemes() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Try both locations
	dirs := []string{
		filepath.Join(homeDir, ".config", "cosa", "themes"),
		filepath.Join(homeDir, ".cosa", "themes"),
	}

	for _, dir := range dirs {
		if err := LoadCustomThemes(dir); err != nil {
			// Continue trying other directories
			continue
		}
	}

	return nil
}

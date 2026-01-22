// Package component provides UI components for the TUI.
package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"cosa/internal/tui/theme"
)

// TemplateItem represents a template in the selector.
type TemplateItem struct {
	ID          string
	Name        string
	Description string
	Type        string
	Variables   []TemplateVariable
}

// TemplateVariable represents a variable in a template.
type TemplateVariable struct {
	Name        string
	Description string
	Required    bool
	Default     string
	Value       string
}

// TemplateSelector is a template selection component.
type TemplateSelector struct {
	templates      []TemplateItem
	filtered       []TemplateItem
	selected       int
	visible        bool
	width          int
	maxHeight      int
	filterInput    *Input
	phase          templatePhase
	selectedTmpl   *TemplateItem
	variableInputs []*Input
	variableIdx    int
	onSelect       func(templateID string, variables map[string]string)
	onCancel       func()
}

type templatePhase int

const (
	phaseSelectTemplate templatePhase = iota
	phaseEnterVariables
)

// NewTemplateSelector creates a new template selector.
func NewTemplateSelector() *TemplateSelector {
	ts := &TemplateSelector{
		templates:   make([]TemplateItem, 0),
		filtered:    make([]TemplateItem, 0),
		filterInput: NewInput(),
		width:       70,
		maxHeight:   20,
		phase:       phaseSelectTemplate,
	}
	ts.filterInput.SetPlaceholder("Filter templates...")
	return ts
}

// SetTemplates sets the available templates.
func (ts *TemplateSelector) SetTemplates(templates []TemplateItem) {
	ts.templates = templates
	ts.filtered = templates
}

// SetSize sets the selector dimensions.
func (ts *TemplateSelector) SetSize(width, maxHeight int) {
	ts.width = width
	ts.maxHeight = maxHeight
	ts.filterInput.SetWidth(width - 6)
}

// SetOnSelect sets the callback for template selection.
func (ts *TemplateSelector) SetOnSelect(fn func(templateID string, variables map[string]string)) {
	ts.onSelect = fn
}

// SetOnCancel sets the callback for cancellation.
func (ts *TemplateSelector) SetOnCancel(fn func()) {
	ts.onCancel = fn
}

// Show makes the selector visible.
func (ts *TemplateSelector) Show() {
	ts.visible = true
	ts.phase = phaseSelectTemplate
	ts.filterInput.Reset()
	ts.filterInput.Focus()
	ts.selected = 0
	ts.filtered = ts.templates
	ts.selectedTmpl = nil
	ts.variableInputs = nil
	ts.variableIdx = 0
}

// Hide hides the selector.
func (ts *TemplateSelector) Hide() {
	ts.visible = false
	ts.filterInput.Reset()
}

// Visible returns true if the selector is visible.
func (ts *TemplateSelector) Visible() bool {
	return ts.visible
}

// HandleKey handles key presses.
func (ts *TemplateSelector) HandleKey(key string) string {
	if ts.phase == phaseSelectTemplate {
		return ts.handleTemplateSelection(key)
	}
	return ts.handleVariableInput(key)
}

func (ts *TemplateSelector) handleTemplateSelection(key string) string {
	switch key {
	case "esc":
		ts.Hide()
		if ts.onCancel != nil {
			ts.onCancel()
		}
		return "cancel"
	case "enter":
		if ts.selected >= 0 && ts.selected < len(ts.filtered) {
			ts.selectedTmpl = &ts.filtered[ts.selected]
			if len(ts.selectedTmpl.Variables) > 0 {
				ts.initVariableInputs()
				ts.phase = phaseEnterVariables
				return ""
			}
			// No variables, select immediately
			if ts.onSelect != nil {
				ts.onSelect(ts.selectedTmpl.ID, make(map[string]string))
			}
			ts.Hide()
			return "select"
		}
		return ""
	case "up", "ctrl+p":
		if ts.selected > 0 {
			ts.selected--
		}
		return ""
	case "down", "ctrl+n":
		if ts.selected < len(ts.filtered)-1 {
			ts.selected++
		}
		return ""
	default:
		ts.filterInput.HandleKey(key)
		ts.filterTemplates()
		return ""
	}
}

func (ts *TemplateSelector) handleVariableInput(key string) string {
	switch key {
	case "esc":
		ts.phase = phaseSelectTemplate
		ts.filterInput.Focus()
		return ""
	case "tab", "down":
		if ts.variableIdx < len(ts.variableInputs)-1 {
			ts.variableInputs[ts.variableIdx].Blur()
			ts.variableIdx++
			ts.variableInputs[ts.variableIdx].Focus()
		}
		return ""
	case "shift+tab", "up":
		if ts.variableIdx > 0 {
			ts.variableInputs[ts.variableIdx].Blur()
			ts.variableIdx--
			ts.variableInputs[ts.variableIdx].Focus()
		}
		return ""
	case "enter", "ctrl+enter":
		// Collect variables and submit
		vars := make(map[string]string)
		for i, v := range ts.selectedTmpl.Variables {
			val := ts.variableInputs[i].Value()
			if val == "" && v.Default != "" {
				val = v.Default
			}
			if val != "" {
				vars[v.Name] = val
			}
		}
		if ts.onSelect != nil {
			ts.onSelect(ts.selectedTmpl.ID, vars)
		}
		ts.Hide()
		return "select"
	default:
		if ts.variableIdx >= 0 && ts.variableIdx < len(ts.variableInputs) {
			ts.variableInputs[ts.variableIdx].HandleKey(key)
		}
		return ""
	}
}

func (ts *TemplateSelector) initVariableInputs() {
	ts.variableInputs = make([]*Input, len(ts.selectedTmpl.Variables))
	for i, v := range ts.selectedTmpl.Variables {
		input := NewInput()
		input.SetWidth(ts.width - 10)
		if v.Default != "" {
			input.SetPlaceholder(v.Default)
		} else if !v.Required {
			input.SetPlaceholder("(optional)")
		}
		ts.variableInputs[i] = input
	}
	ts.variableIdx = 0
	if len(ts.variableInputs) > 0 {
		ts.variableInputs[0].Focus()
	}
}

func (ts *TemplateSelector) filterTemplates() {
	query := strings.ToLower(ts.filterInput.Value())
	if query == "" {
		ts.filtered = ts.templates
		ts.selected = 0
		return
	}

	ts.filtered = make([]TemplateItem, 0)
	for _, tmpl := range ts.templates {
		name := strings.ToLower(tmpl.Name)
		desc := strings.ToLower(tmpl.Description)
		typ := strings.ToLower(tmpl.Type)
		if strings.Contains(name, query) || strings.Contains(desc, query) || strings.Contains(typ, query) {
			ts.filtered = append(ts.filtered, tmpl)
		}
	}

	if ts.selected >= len(ts.filtered) {
		ts.selected = len(ts.filtered) - 1
	}
	if ts.selected < 0 {
		ts.selected = 0
	}
}

// View renders the template selector.
func (ts *TemplateSelector) View() string {
	if !ts.visible {
		return ""
	}

	if ts.phase == phaseSelectTemplate {
		return ts.viewTemplateList()
	}
	return ts.viewVariableInput()
}

func (ts *TemplateSelector) viewTemplateList() string {
	t := theme.Current

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		MarginBottom(1)
	title := titleStyle.Render("Select Template")

	// Filter input
	inputView := ts.filterInput.View()

	// Template list
	listHeight := ts.maxHeight - 6
	if listHeight < 3 {
		listHeight = 3
	}
	if listHeight > len(ts.filtered) {
		listHeight = len(ts.filtered)
	}

	var tmplLines []string
	for i, tmpl := range ts.filtered {
		if len(tmplLines) >= listHeight {
			break
		}
		line := ts.renderTemplateLine(tmpl, i == ts.selected)
		tmplLines = append(tmplLines, line)
	}

	if len(ts.filtered) == 0 {
		noResults := lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Padding(0, 2).
			Render("No templates found")
		tmplLines = append(tmplLines, noResults)
	}

	templateList := strings.Join(tmplLines, "\n")

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		MarginTop(1)
	help := helpStyle.Render("↑/↓ select • Enter confirm • Esc cancel")

	// Container
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		inputView,
		templateList,
		help,
	)

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderActive).
		Background(t.Background).
		Width(ts.width).
		Padding(1)

	return containerStyle.Render(content)
}

func (ts *TemplateSelector) viewVariableInput() string {
	t := theme.Current

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		MarginBottom(1)
	title := titleStyle.Render("Configure: " + ts.selectedTmpl.Name)

	// Variable inputs
	var varLines []string
	for i, v := range ts.selectedTmpl.Variables {
		labelStyle := lipgloss.NewStyle().
			Foreground(t.Text)
		if v.Required {
			labelStyle = labelStyle.Bold(true)
		}

		label := v.Name
		if v.Required {
			label += " *"
		}
		varLines = append(varLines, labelStyle.Render(label))

		descStyle := lipgloss.NewStyle().
			Foreground(t.TextMuted).
			MarginBottom(0)
		if v.Description != "" {
			varLines = append(varLines, descStyle.Render(v.Description))
		}

		if ts.variableInputs != nil && i < len(ts.variableInputs) {
			varLines = append(varLines, ts.variableInputs[i].View())
		}
		varLines = append(varLines, "") // spacing
	}

	variableSection := strings.Join(varLines, "\n")

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		MarginTop(1)
	help := helpStyle.Render("Tab next • Enter submit • Esc back")

	// Container
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		variableSection,
		help,
	)

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderActive).
		Background(t.Background).
		Width(ts.width).
		Padding(1)

	return containerStyle.Render(content)
}

func (ts *TemplateSelector) renderTemplateLine(tmpl TemplateItem, selected bool) string {
	t := theme.Current

	var nameStyle, descStyle, typeStyle lipgloss.Style

	if selected {
		nameStyle = lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
		descStyle = lipgloss.NewStyle().
			Foreground(t.Text)
		typeStyle = lipgloss.NewStyle().
			Foreground(t.Secondary)
	} else {
		nameStyle = lipgloss.NewStyle().
			Foreground(t.Text)
		descStyle = lipgloss.NewStyle().
			Foreground(t.TextMuted)
		typeStyle = lipgloss.NewStyle().
			Foreground(t.TextMuted)
	}

	// Selection indicator
	indicator := "  "
	if selected {
		indicator = lipgloss.NewStyle().
			Foreground(t.Primary).
			Render("▸ ")
	}

	// Type badge
	typeBadge := typeStyle.Render("[" + tmpl.Type + "]")

	// Build line
	name := nameStyle.Render(tmpl.Name)

	// Truncate description if needed
	available := ts.width - lipgloss.Width(indicator) - lipgloss.Width(name) - lipgloss.Width(typeBadge) - 8
	desc := tmpl.Description
	if len(desc) > available && available > 3 {
		desc = desc[:available-3] + "..."
	}
	descView := descStyle.Render(" - " + desc)

	return indicator + name + descView + " " + typeBadge
}

// CenterIn returns the selector centered in the given dimensions.
func (ts *TemplateSelector) CenterIn(screenWidth, screenHeight int) string {
	selector := ts.View()
	if selector == "" {
		return ""
	}

	selectorWidth := lipgloss.Width(selector)
	selectorHeight := lipgloss.Height(selector)

	// Center
	padLeft := (screenWidth - selectorWidth) / 2
	padTop := (screenHeight - selectorHeight) / 2

	if padLeft < 0 {
		padLeft = 0
	}
	if padTop < 0 {
		padTop = 0
	}

	return lipgloss.NewStyle().
		PaddingLeft(padLeft).
		PaddingTop(padTop).
		Render(selector)
}

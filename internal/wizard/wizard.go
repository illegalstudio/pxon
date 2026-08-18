package wizard

import (
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pxon/internal/theme"
)

var (
	labelStyle    = lipgloss.NewStyle().Bold(true).Foreground(theme.TitleColor)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.SuccessColor)
	cursorStyle   = lipgloss.NewStyle().Foreground(theme.LabelColor)
	dimStyle      = lipgloss.NewStyle().Foreground(theme.MutedColor)
	errorStyle    = lipgloss.NewStyle().Foreground(theme.ErrorColor)
)

// ErrCancelled is returned when the user presses Ctrl+C.
var ErrCancelled = errors.New("cancelled")

// ---- Select ----

type selectModel struct {
	label    string
	options  []string
	cursor   int
	selected int
	done     bool
	quit     bool
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quit = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "tab":
			if len(m.options) > 0 {
				m.cursor = (m.cursor + 1) % len(m.options)
			}
		case "shift+tab":
			if len(m.options) > 0 {
				m.cursor = (m.cursor - 1 + len(m.options)) % len(m.options)
			}
		case "enter":
			m.selected = m.cursor
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m selectModel) View() string {
	var sb strings.Builder
	sb.WriteString(labelStyle.Render(m.label))
	sb.WriteString("\n")
	for i, opt := range m.options {
		if i == m.cursor {
			sb.WriteString(cursorStyle.Render("▸ "))
			sb.WriteString(selectedStyle.Render(opt))
		} else {
			sb.WriteString("  ")
			sb.WriteString(dimStyle.Render(opt))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(dimStyle.Render("↑/↓ or tab/shift+tab navigate  •  enter to confirm"))
	return sb.String()
}

// Select shows an interactive list picker and returns the chosen option.
func Select(label string, options []string, defaultIndex int) (string, error) {
	if defaultIndex < 0 || defaultIndex >= len(options) {
		defaultIndex = 0
	}
	p := tea.NewProgram(selectModel{label: label, options: options, cursor: defaultIndex})
	result, err := p.Run()
	if err != nil {
		return "", err
	}
	m := result.(selectModel)
	if m.quit {
		return "", ErrCancelled
	}
	return options[m.selected], nil
}

// Confirm asks a yes/no question and returns true when the user selects "Yes".
func Confirm(label string, defaultYes bool) (bool, error) {
	options := []string{"No", "Yes"}
	defaultIndex := 0
	if defaultYes {
		defaultIndex = 1
	}

	choice, err := Select(label, options, defaultIndex)
	if err != nil {
		return false, err
	}

	return choice == "Yes", nil
}

// ---- MultiSelect ----

type multiSelectModel struct {
	label    string
	options  []string
	cursor   int
	selected map[int]bool
	done     bool
	quit     bool
	errMsg   string
}

func (m multiSelectModel) Init() tea.Cmd { return nil }

func (m multiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.selected == nil {
		m.selected = make(map[int]bool)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quit = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "tab":
			if len(m.options) > 0 {
				m.cursor = (m.cursor + 1) % len(m.options)
			}
		case "shift+tab":
			if len(m.options) > 0 {
				m.cursor = (m.cursor - 1 + len(m.options)) % len(m.options)
			}
		case " ":
			if len(m.options) > 0 {
				m.selected[m.cursor] = !m.selected[m.cursor]
				m.errMsg = ""
			}
		case "enter":
			if selectedCount(m.selected) == 0 {
				m.errMsg = "Select at least one destination."
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m multiSelectModel) View() string {
	var sb strings.Builder
	sb.WriteString(labelStyle.Render(m.label))
	sb.WriteString("\n")
	for i, option := range m.options {
		if i == m.cursor {
			sb.WriteString(cursorStyle.Render("▸ "))
		} else {
			sb.WriteString("  ")
		}

		checkbox := "[ ] "
		style := dimStyle
		if m.selected[i] {
			checkbox = "[x] "
			style = selectedStyle
		} else if i == m.cursor {
			style = selectedStyle
		}
		sb.WriteString(style.Render(checkbox + option))
		sb.WriteString("\n")
	}
	if m.errMsg != "" {
		sb.WriteString(errorStyle.Render(m.errMsg))
		sb.WriteString("\n")
	}
	sb.WriteString(dimStyle.Render("↑/↓ or tab/shift+tab navigate  •  space toggles  •  enter confirms"))
	return sb.String()
}

// MultiSelect shows an interactive list picker and returns the selected options
// in their original order.
func MultiSelect(label string, options []string, defaultSelected []int) ([]string, error) {
	selected := make(map[int]bool)
	for _, index := range defaultSelected {
		if index >= 0 && index < len(options) {
			selected[index] = true
		}
	}

	p := tea.NewProgram(multiSelectModel{label: label, options: options, selected: selected})
	result, err := p.Run()
	if err != nil {
		return nil, err
	}
	m := result.(multiSelectModel)
	if m.quit {
		return nil, ErrCancelled
	}

	choices := make([]string, 0, selectedCount(m.selected))
	for index, option := range options {
		if m.selected[index] {
			choices = append(choices, option)
		}
	}
	return choices, nil
}

func selectedCount(selected map[int]bool) int {
	count := 0
	for _, active := range selected {
		if active {
			count++
		}
	}
	return count
}

// ---- Input ----

type inputModel struct {
	label    string
	ti       textinput.Model
	required bool
	done     bool
	quit     bool
	errMsg   string
}

func (m inputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quit = true
			return m, tea.Quit
		case "enter":
			if m.required && strings.TrimSpace(m.ti.Value()) == "" {
				m.errMsg = "A value is required."
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		default:
			m.errMsg = ""
		}
	}
	var cmd tea.Cmd
	m.ti, cmd = m.ti.Update(msg)
	return m, cmd
}

func (m inputModel) View() string {
	var sb strings.Builder
	sb.WriteString(labelStyle.Render(m.label))
	sb.WriteString("\n")
	sb.WriteString(m.ti.View())
	if m.errMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(errorStyle.Render(m.errMsg))
	}
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("enter to confirm"))
	return sb.String()
}

// Input shows a text input prompt and returns the entered value.
// If defaultValue is non-empty it is pre-filled in the field.
func Input(label, defaultValue string, required bool) (string, error) {
	ti := textinput.New()
	ti.PromptStyle = lipgloss.NewStyle().Foreground(theme.LabelColor)
	ti.TextStyle = lipgloss.NewStyle().Foreground(theme.TextColor)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(theme.MutedColor)
	ti.CompletionStyle = lipgloss.NewStyle().Foreground(theme.MutedColor)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(theme.LabelColor)
	ti.Cursor.TextStyle = lipgloss.NewStyle().Foreground(theme.TextColor)
	ti.SetValue(defaultValue)
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	p := tea.NewProgram(inputModel{label: label, ti: ti, required: required})
	result, err := p.Run()
	if err != nil {
		return "", err
	}
	m := result.(inputModel)
	if m.quit {
		return "", ErrCancelled
	}
	return strings.TrimSpace(m.ti.Value()), nil
}

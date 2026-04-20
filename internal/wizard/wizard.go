package wizard

import (
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	labelStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// ErrCancelled is returned when the user presses Ctrl+C.
var ErrCancelled = errors.New("annullato")

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
	sb.WriteString(dimStyle.Render("↑/↓ naviga  •  invio conferma"))
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
				m.errMsg = "Valore richiesto."
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
	sb.WriteString(dimStyle.Render("invio conferma"))
	return sb.String()
}

// Input shows a text input prompt and returns the entered value.
// If defaultValue is non-empty it is pre-filled in the field.
func Input(label, defaultValue string, required bool) (string, error) {
	ti := textinput.New()
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

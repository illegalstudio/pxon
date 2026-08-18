package wizard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSelectModelTabNavigation(t *testing.T) {
	tests := []struct {
		name       string
		cursor     int
		key        tea.KeyType
		wantCursor int
	}{
		{name: "tab advances", cursor: 0, key: tea.KeyTab, wantCursor: 1},
		{name: "tab wraps", cursor: 1, key: tea.KeyTab, wantCursor: 0},
		{name: "shift tab goes back", cursor: 1, key: tea.KeyShiftTab, wantCursor: 0},
		{name: "shift tab wraps", cursor: 0, key: tea.KeyShiftTab, wantCursor: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := selectModel{
				options: []string{"No", "Yes"},
				cursor:  tt.cursor,
			}

			updated, _ := model.Update(tea.KeyMsg{Type: tt.key})
			got := updated.(selectModel)
			if got.cursor != tt.wantCursor {
				t.Fatalf("cursor = %d, want %d", got.cursor, tt.wantCursor)
			}
		})
	}
}

func TestMultiSelectModelTogglesOptions(t *testing.T) {
	model := multiSelectModel{
		options:  []string{"~/.agents/skills", "~/.claude/skills"},
		selected: map[int]bool{0: true, 1: true},
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	got := updated.(multiSelectModel)
	if got.selected[0] {
		t.Fatal("first option remained selected after space")
	}
	if !got.selected[1] {
		t.Fatal("second option was unexpectedly deselected")
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = updated.(multiSelectModel)
	if got.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", got.cursor)
	}
}

func TestMultiSelectModelRequiresSelection(t *testing.T) {
	model := multiSelectModel{options: []string{"~/.agents/skills", "~/.claude/skills"}}

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(multiSelectModel)
	if command != nil {
		t.Fatal("empty selection unexpectedly quit")
	}
	if got.done {
		t.Fatal("empty selection was accepted")
	}
	if got.errMsg == "" {
		t.Fatal("empty selection did not report an error")
	}
}

func TestMultiSelectModelAcceptsSelection(t *testing.T) {
	model := multiSelectModel{
		options:  []string{"~/.agents/skills", "~/.claude/skills"},
		selected: map[int]bool{0: true},
	}

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(multiSelectModel)
	if command == nil {
		t.Fatal("selected options did not quit")
	}
	if !got.done {
		t.Fatal("selected options were not accepted")
	}
}

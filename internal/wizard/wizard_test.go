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

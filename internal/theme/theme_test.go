package theme

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPaletteContrast(t *testing.T) {
	colors := map[string]lipgloss.AdaptiveColor{
		"title":   TitleColor,
		"label":   LabelColor,
		"text":    TextColor,
		"muted":   MutedColor,
		"success": SuccessColor,
		"error":   ErrorColor,
	}

	for name, color := range colors {
		t.Run(name+"/light", func(t *testing.T) {
			assertContrast(t, color.Light, "#FFFFFF")
		})
		t.Run(name+"/dark", func(t *testing.T) {
			assertContrast(t, color.Dark, "#000000")
		})
	}
}

func assertContrast(t *testing.T, foreground, background string) {
	t.Helper()

	foregroundLuminance := relativeLuminance(t, foreground)
	backgroundLuminance := relativeLuminance(t, background)
	lighter := math.Max(foregroundLuminance, backgroundLuminance)
	darker := math.Min(foregroundLuminance, backgroundLuminance)
	ratio := (lighter + 0.05) / (darker + 0.05)

	if ratio < 4.5 {
		t.Fatalf("contrast ratio for %s on %s is %.2f, want at least 4.5", foreground, background, ratio)
	}
}

func relativeLuminance(t *testing.T, color string) float64 {
	t.Helper()

	hex := strings.TrimPrefix(color, "#")
	if len(hex) != 6 {
		t.Fatalf("invalid color %q", color)
	}

	components := make([]float64, 3)
	for i := range components {
		value, err := strconv.ParseUint(hex[i*2:i*2+2], 16, 8)
		if err != nil {
			t.Fatalf("invalid color %q: %v", color, err)
		}

		channel := float64(value) / 255
		if channel <= 0.04045 {
			components[i] = channel / 12.92
		} else {
			components[i] = math.Pow((channel+0.055)/1.055, 2.4)
		}
	}

	return 0.2126*components[0] + 0.7152*components[1] + 0.0722*components[2]
}

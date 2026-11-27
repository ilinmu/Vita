package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// Palette holds Vita's semantic colors derived from the active member's accent
// color and fixed clinical/status hues.
type Palette struct {
	Dark         bool
	Teal         ui.Color // Active member primary accent
	TealSoft     ui.Color
	TealBandOut  ui.Color
	TealBandIn   ui.Color
	Indigo       ui.Color
	IndigoSoft   ui.Color
	Amber        ui.Color
	AmberSoft    ui.Color
	Red          ui.Color
	RedSoft      ui.Color
	Green        ui.Color
	GreenSoft    ui.Color
	Card         ui.Color
	CardElevated ui.Color
}

var memberSwatches = []string{
	"#0D9488", // Teal
	"#2563EB", // Blue
	"#E11D48", // Rose
	"#6366F1", // Indigo
	"#D97706", // Amber
	"#059669", // Emerald
}

func parseMemberAccent(hex string) ui.Color {
	s := strings.TrimSpace(hex)
	if s == "" || !strings.HasPrefix(s, "#") || (len(s) != 7 && len(s) != 4) {
		return ui.Hex("#0D9488")
	}
	c := ui.Hex(s)
	if c.A == 0 {
		return ui.Hex("#0D9488")
	}
	return c
}

func contrastText(bg ui.Color) ui.Color {
	// Relative perceived brightness
	lum := 0.299*float64(bg.R) + 0.587*float64(bg.G) + 0.114*float64(bg.B)
	if lum > 165 {
		return ui.Hex("#0F172A")
	}
	return ui.RGB(255, 255, 255)
}

func resolveTheme(c *ui.Context, mode string, memberColorHex string) (*ui.Theme, Palette) {
	sysDark := c.Theme().Dark
	useDark := sysDark
	switch mode {
	case "light":
		useDark = false
	case "dark":
		useDark = true
	}

	base := parseMemberAccent(memberColorHex)

	if useDark {
		accent := base.Mix(ui.RGB(255, 255, 255), 0.28)
		accentHover := base.Mix(ui.RGB(255, 255, 255), 0.40)
		accentPressed := base.Mix(ui.RGB(255, 255, 255), 0.15)

		t := ui.DarkTheme()
		t.Background = ui.Hex("#0B1120")
		t.Surface = ui.Hex("#151F32")
		t.SurfaceHover = ui.Hex("#1E293B")
		t.SurfacePressed = ui.Hex("#273549")
		t.Border = ui.Hex("#263449")
		t.Text = ui.Hex("#F1F5F9")
		t.TextMuted = ui.Hex("#94A3B8")
		t.Accent = accent
		t.AccentHover = accentHover
		t.AccentPressed = accentPressed
		t.AccentText = contrastText(accent)
		t.Selection = accent.Alpha(0.32)
		t.Focus = accent.Alpha(0.55)
		t.Danger = ui.Hex("#F87171")
		t.Warning = ui.Hex("#FBBF24")
		t.Success = ui.Hex("#4ADE80")
		t.Radius = 10
		return t, Palette{
			Dark:         true,
			Teal:         accent,
			TealSoft:     accent.Alpha(0.16),
			TealBandOut:  accent.Alpha(0.10),
			TealBandIn:   accent.Alpha(0.18),
			Indigo:       ui.Hex("#818CF8"),
			IndigoSoft:   ui.RGBA(129, 140, 248, 0.16),
			Amber:        ui.Hex("#FBBF24"),
			AmberSoft:    ui.RGBA(251, 191, 36, 0.15),
			Red:          ui.Hex("#F87171"),
			RedSoft:      ui.RGBA(248, 113, 113, 0.16),
			Green:        ui.Hex("#4ADE80"),
			GreenSoft:    ui.RGBA(74, 222, 128, 0.14),
			Card:         ui.Hex("#151F32"),
			CardElevated: ui.Hex("#1E293B"),
		}
	}

	accent := base
	accentHover := base.Mix(ui.RGB(0, 0, 0), 0.14)
	accentPressed := base.Mix(ui.RGB(0, 0, 0), 0.24)

	t := ui.LightTheme()
	t.Background = ui.Hex("#F8FAFC")
	t.Surface = ui.Hex("#FFFFFF")
	t.SurfaceHover = ui.Hex("#F1F5F9")
	t.SurfacePressed = ui.Hex("#E2E8F0")
	t.Border = ui.Hex("#E2E8F0")
	t.Text = ui.Hex("#0F172A")
	t.TextMuted = ui.Hex("#64748B")
	t.Accent = accent
	t.AccentHover = accentHover
	t.AccentPressed = accentPressed
	t.AccentText = contrastText(accent)
	t.Selection = accent.Alpha(0.24)
	t.Focus = accent.Alpha(0.50)
	t.Danger = ui.Hex("#EF4444")
	t.Warning = ui.Hex("#F59E0B")
	t.Success = ui.Hex("#16A34A")
	t.Radius = 10
	return t, Palette{
		Dark:         false,
		Teal:         accent,
		TealSoft:     accent.Alpha(0.11),
		TealBandOut:  accent.Alpha(0.08),
		TealBandIn:   accent.Alpha(0.16),
		Indigo:       ui.Hex("#6366F1"),
		IndigoSoft:   ui.RGBA(99, 102, 241, 0.11),
		Amber:        ui.Hex("#D97706"),
		AmberSoft:    ui.RGBA(245, 158, 11, 0.12),
		Red:          ui.Hex("#DC2626"),
		RedSoft:      ui.RGBA(239, 68, 68, 0.12),
		Green:        ui.Hex("#16A34A"),
		GreenSoft:    ui.RGBA(22, 163, 74, 0.11),
		Card:         ui.Hex("#FFFFFF"),
		CardElevated: ui.Hex("#F1F5F9"),
	}
}

func tempColor(tempC float64, pal Palette) ui.Color {
	switch {
	case tempC >= 38.5:
		return pal.Red
	case tempC >= 37.3:
		return pal.Amber
	default:
		return pal.Green
	}
}

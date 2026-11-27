package app

import (
	"github.com/egoist/mygo/ui"
)

// Palette holds Vita's semantic colors derived from the active light or dark theme.
type Palette struct {
	Dark         bool
	Teal         ui.Color
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
	"#6366F1", // Indigo
	"#EC4899", // Rose
	"#F59E0B", // Amber
	"#3B82F6", // Blue
	"#10B981", // Emerald
}

func resolveTheme(c *ui.Context, mode string) (*ui.Theme, Palette) {
	sysDark := c.Theme().Dark
	useDark := sysDark
	switch mode {
	case "light":
		useDark = false
	case "dark":
		useDark = true
	}

	if useDark {
		t := ui.DarkTheme()
		t.Background = ui.Hex("#0B1120")
		t.Surface = ui.Hex("#151F32")
		t.SurfaceHover = ui.Hex("#1E293B")
		t.SurfacePressed = ui.Hex("#273549")
		t.Border = ui.Hex("#263449")
		t.Text = ui.Hex("#F1F5F9")
		t.TextMuted = ui.Hex("#94A3B8")
		t.Accent = ui.Hex("#2DD4BF")
		t.AccentHover = ui.Hex("#5EEAD4")
		t.AccentPressed = ui.Hex("#14B8A6")
		t.AccentText = ui.Hex("#042F2E")
		t.Danger = ui.Hex("#F87171")
		t.Warning = ui.Hex("#FBBF24")
		t.Success = ui.Hex("#4ADE80")
		t.Radius = 10
		return t, Palette{
			Dark:         true,
			Teal:         ui.Hex("#2DD4BF"),
			TealSoft:     ui.RGBA(45, 212, 191, 0.14),
			TealBandOut:  ui.RGBA(45, 212, 191, 0.10),
			TealBandIn:   ui.RGBA(45, 212, 191, 0.18),
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

	t := ui.LightTheme()
	t.Background = ui.Hex("#F8FAFC")
	t.Surface = ui.Hex("#FFFFFF")
	t.SurfaceHover = ui.Hex("#F1F5F9")
	t.SurfacePressed = ui.Hex("#E2E8F0")
	t.Border = ui.Hex("#E2E8F0")
	t.Text = ui.Hex("#0F172A")
	t.TextMuted = ui.Hex("#64748B")
	t.Accent = ui.Hex("#0D9488")
	t.AccentHover = ui.Hex("#0F766E")
	t.AccentPressed = ui.Hex("#115E59")
	t.AccentText = ui.Hex("#FFFFFF")
	t.Danger = ui.Hex("#EF4444")
	t.Warning = ui.Hex("#F59E0B")
	t.Success = ui.Hex("#16A34A")
	t.Radius = 10
	return t, Palette{
		Dark:         false,
		Teal:         ui.Hex("#0D9488"),
		TealSoft:     ui.RGBA(13, 148, 136, 0.10),
		TealBandOut:  ui.RGBA(13, 148, 136, 0.08),
		TealBandIn:   ui.RGBA(13, 148, 136, 0.15),
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

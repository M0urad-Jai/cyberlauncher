package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────
//  Colour palette  (dark terminal theme)
// ─────────────────────────────────────────────────────────────

var (
	// Base hues
	colBackground  = lipgloss.Color("#0d1117")
	colSurface     = lipgloss.Color("#161b22")
	colBorder      = lipgloss.Color("#30363d")
	colBorderFocus = lipgloss.Color("#58a6ff")
	colText        = lipgloss.Color("#e6edf3")
	colTextMuted   = lipgloss.Color("#8b949e")
	colTextDim     = lipgloss.Color("#484f58")

	// Accent colours
	colAccentCyan   = lipgloss.Color("#79c0ff")
	colAccentGreen  = lipgloss.Color("#3fb950")
	colAccentYellow = lipgloss.Color("#e3b341")
	colAccentRed    = lipgloss.Color("#ff7b72")
	colAccentPurple = lipgloss.Color("#d2a8ff")
	colAccentOrange = lipgloss.Color("#ffa657")
	colAccentTeal   = lipgloss.Color("#39d353")

	// Category tag colours (cycling)
	catColours = []lipgloss.Color{
		"#79c0ff", "#3fb950", "#e3b341", "#ff7b72",
		"#d2a8ff", "#ffa657", "#39d353", "#f78166",
		"#56d364", "#a5d6ff", "#cae8ff", "#b3d9ff",
		"#ffdf5d", "#ff9bce", "#7ee787",
	}
)

// ─────────────────────────────────────────────────────────────
//  Shared styles
// ─────────────────────────────────────────────────────────────

var (
	StyleNormal = lipgloss.NewStyle().
			Foreground(colText)

	StyleMuted = lipgloss.NewStyle().
			Foreground(colTextMuted)

	StyleDim = lipgloss.NewStyle().
			Foreground(colTextDim)

	StyleBold = lipgloss.NewStyle().
			Foreground(colText).
			Bold(true)

	StyleCyan = lipgloss.NewStyle().
			Foreground(colAccentCyan).
			Bold(true)

	StyleGreen = lipgloss.NewStyle().
			Foreground(colAccentGreen).
			Bold(true)

	StyleYellow = lipgloss.NewStyle().
			Foreground(colAccentYellow).
			Bold(true)

	StyleRed = lipgloss.NewStyle().
			Foreground(colAccentRed).
			Bold(true)

	StylePurple = lipgloss.NewStyle().
			Foreground(colAccentPurple).
			Bold(true)

	StyleOrange = lipgloss.NewStyle().
			Foreground(colAccentOrange).
			Bold(true)

	// ── Title / header ─────────────────────────────────────

	StyleAppTitle = lipgloss.NewStyle().
			Foreground(colAccentCyan).
			Bold(true)

	StyleSubtitle = lipgloss.NewStyle().
			Foreground(colTextMuted).
			Italic(true)

	// ── Boxes / panels ─────────────────────────────────────

	StylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colBorder).
			Padding(0, 1)

	StylePanelFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colBorderFocus).
				Padding(0, 1)

	StyleStatusBar = lipgloss.NewStyle().
			Background(colSurface).
			Foreground(colTextMuted).
			Padding(0, 1)

	// ── List items ─────────────────────────────────────────

	StyleItemNormal = lipgloss.NewStyle().
			Foreground(colText).
			PaddingLeft(2)

	StyleItemSelected = lipgloss.NewStyle().
				Foreground(colAccentCyan).
				Bold(true).
				PaddingLeft(1)

	StyleItemChecked = lipgloss.NewStyle().
				Foreground(colAccentGreen).
				PaddingLeft(2)

	StyleItemNumber = lipgloss.NewStyle().
			Foreground(colTextDim).
			Width(4)

	StyleCheckmark = lipgloss.NewStyle().
			Foreground(colAccentGreen).
			Bold(true)

	StyleTag = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true)

	// ── Badges ─────────────────────────────────────────────

	StyleBadgeSelected = lipgloss.NewStyle().
				Background(colAccentGreen).
				Foreground(colBackground).
				Padding(0, 1).
				Bold(true)

	StyleBadgeInstalled = lipgloss.NewStyle().
				Foreground(colAccentGreen).
				Bold(true)

	StyleBadgeCount = lipgloss.NewStyle().
			Background(colAccentCyan).
			Foreground(colBackground).
			Padding(0, 1).
			Bold(true)

	StyleBadgeCategory = lipgloss.NewStyle().
				Background(colSurface).
				Foreground(colTextMuted).
				Padding(0, 1)

	// ── Progress / spinner ─────────────────────────────────

	StyleSpinner = lipgloss.NewStyle().
			Foreground(colAccentCyan)

	StyleProgressBar = lipgloss.NewStyle().
				Foreground(colAccentGreen)

	StyleProgressTrack = lipgloss.NewStyle().
				Foreground(colBorder)

	// ── Results / log ──────────────────────────────────────

	StyleLogOK   = lipgloss.NewStyle().Foreground(colAccentGreen)
	StyleLogWarn = lipgloss.NewStyle().Foreground(colAccentYellow)
	StyleLogErr  = lipgloss.NewStyle().Foreground(colAccentRed)
	StyleLogInfo = lipgloss.NewStyle().Foreground(colAccentCyan)
	StyleLogStep = lipgloss.NewStyle().Foreground(colTextMuted)

	// ── Footer keys ─────────────────────────────────────────

	StyleKey = lipgloss.NewStyle().
			Background(colSurface).
			Foreground(colAccentCyan).
			Padding(0, 1).
			Bold(true)

	StyleKeyDesc = lipgloss.NewStyle().
			Foreground(colTextMuted)

	// ── Separator ──────────────────────────────────────────

	StyleSep = lipgloss.NewStyle().
			Foreground(colBorder)
)

// ─────────────────────────────────────────────────────────────
//  Category colour
// ─────────────────────────────────────────────────────────────

// CatColour returns a distinct colour for a category index.
func CatColour(idx int) lipgloss.Color {
	return catColours[idx%len(catColours)]
}

// CatTag renders a small coloured badge with the category name.
func CatTag(name string, idx int) string {
	return StyleTag.
		Background(CatColour(idx)).
		Foreground(colBackground).
		Render(name)
}

// ─────────────────────────────────────────────────────────────
//  Banner
// ─────────────────────────────────────────────────────────────

// Banner returns the ASCII art header string.
func Banner() string {
	top    := StyleSep.Render("╔══════════════════════════════════════════════════════════════╗")
	mid1   := StyleSep.Render("║") + "  " +
		StyleAppTitle.Render("⚡  CyberLauncher") + " " +
		StyleMuted.Render("—") + " " +
		StyleNormal.Render("Kali Tools Desktop Entry Generator") + "  " +
		StyleSep.Render("║")
	mid2   := StyleSep.Render("║  ") +
		StyleMuted.Render("XFCE / GNOME  •  Arch / Debian  •  Any shell / terminal") +
		StyleSep.Render("  ║")
	bottom := StyleSep.Render("╚══════════════════════════════════════════════════════════════╝")

	return top + "\n" + mid1 + "\n" + mid2 + "\n" + bottom
}

// ─────────────────────────────────────────────────────────────
//  Progress bar
// ─────────────────────────────────────────────────────────────

// ProgressBar renders a simple text progress bar.
func ProgressBar(current, total, width int) string {
	if total == 0 {
		return ""
	}
	filled := (current * width) / total
	if filled > width {
		filled = width
	}
	bar := StyleProgressBar.Render(strings.Repeat("█", filled)) +
		StyleProgressTrack.Render(strings.Repeat("░", width-filled))
	pct := (current * 100) / total
	return bar + " " + StyleMuted.Render(fmt.Sprintf("%d%%", pct))
}

// ─────────────────────────────────────────────────────────────
//  Key help row
// ─────────────────────────────────────────────────────────────

type KeyHint struct {
	Key  string
	Desc string
}

// KeyBar renders a row of keyboard hints.
func KeyBar(hints []KeyHint) string {
	parts := make([]string, 0, len(hints)*2)
	for _, h := range hints {
		parts = append(parts,
			StyleKey.Render(h.Key),
			StyleKeyDesc.Render(h.Desc),
		)
	}
	return strings.Join(parts, " ")
}

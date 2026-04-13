package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cyberlauncher/cyberlauncher/internal/catalogue"
	"github.com/cyberlauncher/cyberlauncher/internal/desktop"
	"github.com/cyberlauncher/cyberlauncher/internal/installer"
	"github.com/cyberlauncher/cyberlauncher/internal/system"
)

// ─────────────────────────────────────────────────────────────
//  Screen identifiers
// ─────────────────────────────────────────────────────────────

type screen int

const (
	screenWelcome    screen = iota // splash / settings
	screenCategories               // category picker
	screenTools                    // tool picker within a category
	screenCustom                   // custom tool wizard
	screenConfirm                  // review selection before processing
	screenProcessing               // live install + .desktop generation
	screenDone                     // summary
)

// ─────────────────────────────────────────────────────────────
//  Message types
// ─────────────────────────────────────────────────────────────

// processDoneMsg signals that all tools have been processed.
type processDoneMsg struct {
	created []string
	skipped []string
}

// processNextMsg carries the pipeline state for the next tool to process.
// The Update loop handles one tool per message, keeping the TUI responsive.
type processNextMsg struct {
	state processState
}

// processState is passed between sequential processNextMsg messages.
type processState struct {
	tools      []catalogue.Tool
	index      int
	noInstall  bool
	si         system.Info
	terminal   string
	shell      string
	desktopDir string
	iconDir    string
	created    []string
	skipped    []string
}

// ─────────────────────────────────────────────────────────────
//  Custom tool form steps
// ─────────────────────────────────────────────────────────────

const (
	customStepName     = iota // 0
	customStepDisplay         // 1
	customStepCmd             // 2
	customStepCategory        // 3
	customStepDesc            // 4
	customStepPkgArch         // 5
	customStepPkgDeb          // 6
	customStepSlug            // 7  — last; on enter adds tool and returns to categories
)

// ─────────────────────────────────────────────────────────────
//  Model
// ─────────────────────────────────────────────────────────────

type Model struct {
	// System info
	sysInfo   system.Info
	noInstall bool

	// User-selected terminal / shell
	terminal string
	shell    string

	// Catalogue data
	categories []string
	byCategory map[string][]catalogue.Tool
	allTools   []catalogue.Tool

	// Navigation state
	activeScreen   screen
	catCursor      int
	toolCursor     int
	settingsCursor int

	// Selection — map of tool.Name → Tool
	selectedTools map[string]catalogue.Tool

	// Settings cycle items
	settingsItems []settingItem

	// Custom tool wizard
	customStep   int
	customInputs [8]textinput.Model
	customTool   catalogue.Tool

	// Terminal dimensions
	width  int
	height int

	// Processing / log state
	spinner           spinner.Model
	logLines          []logLine
	currentTask       string
	progress          int
	total             int
	created           []string
	skipped           []string
	processing        bool
	stopSudoKeepAlive func() // cancels the background sudo-refresh ticker

	// Paths
	desktopDir string
	iconDir    string

	// Tool-screen search / filter
	searchInput textinput.Model
	searching   bool
	filtered    []catalogue.Tool
}

type logLine struct {
	icon  string
	msg   string
	style lipgloss.Style
}

type settingItem struct {
	label   string
	value   *string
	options []string
}

// ─────────────────────────────────────────────────────────────
//  Constructor
// ─────────────────────────────────────────────────────────────

func NewModel(noInstall bool) Model {
	si := system.Detect()
	home := si.HomeDir

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = StyleSpinner

	search := textinput.New()
	search.Placeholder = "search tools…"
	search.PromptStyle = StyleCyan
	search.TextStyle = StyleNormal
	search.CharLimit = 40

	var inputs [8]textinput.Model
	placeholders := []string{
		"unique-id  (e.g. my-scanner)",
		"Display Name  (e.g. My Scanner)",
		"Command  (e.g. myscanner --interactive)",
		"Category number  (enter to list)",
		"Short description  (optional)",
		"Arch / AUR package name  (optional)",
		"Debian / apt package name  (optional)",
		"kali.org/tools/<slug>  (optional, for icon)",
	}
	for i := range inputs {
		t := textinput.New()
		t.Placeholder = placeholders[i]
		t.PromptStyle = StylePurple
		t.TextStyle = StyleNormal
		t.CharLimit = 80
		inputs[i] = t
	}

	// settings items share pointers into the Model so mutations are reflected
	m := Model{
		sysInfo:       si,
		noInstall:     noInstall,
		terminal:      si.Terminal,
		shell:         si.Shell,
		categories:    catalogue.Categories(),
		byCategory:    catalogue.ByCategory(),
		allTools:      catalogue.AllTools(),
		activeScreen:  screenWelcome,
		selectedTools: make(map[string]catalogue.Tool),
		spinner:       sp,
		searchInput:   search,
		customInputs:  inputs,
		desktopDir:    filepath.Join(home, ".local/share/applications/cyber-launcher"),
		iconDir:       filepath.Join(home, ".local/share/cyber-launcher/icons"),
		width:         120,
		height:        40,
	}

	m.settingsItems = []settingItem{
		{label: "Terminal", value: &m.terminal, options: system.KnownTerminals()},
		{label: "Shell", value: &m.shell, options: system.KnownShells()},
	}

	return m
}

// ─────────────────────────────────────────────────────────────
//  Init
// ─────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, textinput.Blink)
}

// ─────────────────────────────────────────────────────────────
//  Update
// ─────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── Terminal resize ──────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	// ── Spinner tick ─────────────────────────────────────
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	// ── Sequential tool processing ───────────────────────
	case processNextMsg:
		state := msg.state
		if state.index >= len(state.tools) {
			// All tools done — refresh desktop DB, then signal done
			desktop.RefreshDB(state.si.HomeDir)
			if m.stopSudoKeepAlive != nil {
				m.stopSudoKeepAlive()
				m.stopSudoKeepAlive = nil
			}
			return m, func() tea.Msg {
				return processDoneMsg{created: state.created, skipped: state.skipped}
			}
		}

		t := &state.tools[state.index]
		m.currentTask = t.DisplayName

		// Process one tool: install → icon fetch → .desktop write
		entries, ok := processSingleTool(
			t, state.noInstall, state.si,
			state.terminal, state.shell,
			state.desktopDir, state.iconDir,
		)
		m.logLines = append(m.logLines, entries...)
		if len(m.logLines) > 120 {
			m.logLines = m.logLines[len(m.logLines)-120:]
		}

		if ok {
			state.created = append(state.created, t.Name)
		} else {
			state.skipped = append(state.skipped, t.Name)
		}
		m.progress = state.index + 1
		state.index++

		next := state
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg { return processNextMsg{state: next} },
		)

	// ── Processing finished ───────────────────────────────
	case processDoneMsg:
		m.processing = false
		m.created = msg.created
		m.skipped = msg.skipped
		m.activeScreen = screenDone
		return m, nil

	// ── Keyboard ─────────────────────────────────────────
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Delegate to focused text input when searching
	if m.activeScreen == screenTools && m.searching {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.filtered = filterTools(
			m.byCategory[m.categories[m.catCursor]],
			m.searchInput.Value(),
		)
		if m.toolCursor >= len(m.filtered) {
			m.toolCursor = max(0, len(m.filtered)-1)
		}
		return m, cmd
	}
	if m.activeScreen == screenCustom {
		var cmd tea.Cmd
		m.customInputs[m.customStep], cmd = m.customInputs[m.customStep].Update(msg)
		return m, cmd
	}

	return m, nil
}

// ─────────────────────────────────────────────────────────────
//  Key handler
// ─────────────────────────────────────────────────────────────

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.activeScreen {

	// ── Welcome / settings ───────────────────────────────
	case screenWelcome:
		switch key {
		case "up", "k":
			if m.settingsCursor > 0 {
				m.settingsCursor--
			}
		case "down", "j":
			if m.settingsCursor < len(m.settingsItems)-1 {
				m.settingsCursor++
			}
		case "left", "h":
			cycleOption(&m.settingsItems[m.settingsCursor], -1)
		case "right", "l", "enter", " ":
			cycleOption(&m.settingsItems[m.settingsCursor], 1)
		case "tab", "n":
			m.activeScreen = screenCategories
		case "q":
			return m, tea.Quit
		}

	// ── Category list ────────────────────────────────────
	case screenCategories:
		switch key {
		case "up", "k":
			if m.catCursor > 0 {
				m.catCursor--
			}
		case "down", "j":
			if m.catCursor < len(m.categories) { // +1 for "Add Custom" row
				m.catCursor++
			}
		case "enter", " ":
			if m.catCursor == len(m.categories) {
				// "Add Custom Tool" row
				m.activeScreen = screenCustom
				m.customStep = customStepName
				m.customInputs[customStepName].Focus()
			} else {
				m.activeScreen = screenTools
				m.toolCursor = 0
				m.searching = false
				m.searchInput.SetValue("")
				m.filtered = m.byCategory[m.categories[m.catCursor]]
			}
		case "a":
			for _, t := range m.allTools {
				m.selectedTools[t.Name] = t
			}
		case "D":
			m.selectedTools = make(map[string]catalogue.Tool)
		case "tab", "backspace":
			m.activeScreen = screenWelcome
		case "ctrl+p":
			if len(m.selectedTools) > 0 {
				m.activeScreen = screenConfirm
			}
		case "q":
			return m, tea.Quit
		}

	// ── Tool list ────────────────────────────────────────
	case screenTools:
		// While search box is active, delegate to it
		if m.searching {
			switch key {
			case "esc":
				m.searching = false
				m.searchInput.Blur()
				m.searchInput.SetValue("")
				m.filtered = m.byCategory[m.categories[m.catCursor]]
			case "enter":
				m.searching = false
				m.searchInput.Blur()
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.filtered = filterTools(
					m.byCategory[m.categories[m.catCursor]],
					m.searchInput.Value(),
				)
				if m.toolCursor >= len(m.filtered) {
					m.toolCursor = max(0, len(m.filtered)-1)
				}
				return m, cmd
			}
			return m, nil
		}

		switch key {
		case "up", "k":
			if m.toolCursor > 0 {
				m.toolCursor--
			}
		case "down", "j":
			if m.toolCursor < len(m.filtered)-1 {
				m.toolCursor++
			}
		case "enter", " ":
			if len(m.filtered) > 0 {
				t := m.filtered[m.toolCursor]
				if _, ok := m.selectedTools[t.Name]; ok {
					delete(m.selectedTools, t.Name)
				} else {
					m.selectedTools[t.Name] = t
				}
			}
		case "a":
			for _, t := range m.filtered {
				m.selectedTools[t.Name] = t
			}
		case "D":
			for _, t := range m.filtered {
				delete(m.selectedTools, t.Name)
			}
		case "/":
			m.searching = true
			m.searchInput.Focus()
		case "esc", "backspace", "b":
			m.activeScreen = screenCategories
			m.toolCursor = 0
		case "ctrl+p":
			if len(m.selectedTools) > 0 {
				m.activeScreen = screenConfirm
			}
		case "q":
			return m, tea.Quit
		}

	// ── Custom tool wizard ───────────────────────────────
	case screenCustom:
		switch key {
		case "esc":
			// Reset wizard state and go back
			for i := range m.customInputs {
				m.customInputs[i].SetValue("")
				m.customInputs[i].Blur()
			}
			m.customStep = customStepName
			m.customTool = catalogue.Tool{}
			m.activeScreen = screenCategories
		case "enter":
			return m.advanceCustomStep()
		case "shift+tab":
			if m.customStep > 0 {
				m.customInputs[m.customStep].Blur()
				m.customStep--
				m.customInputs[m.customStep].Focus()
			}
		default:
			// All other keys (letters, digits, backspace, space, …) must be
			// forwarded directly to the active textinput — this is the root
			// cause of the "not accepting input" bug: tea.KeyMsg was caught by
			// the outer switch and sent here, but the delegate at the bottom
			// of Update() was never reached for KeyMsg.
			var cmd tea.Cmd
			m.customInputs[m.customStep], cmd = m.customInputs[m.customStep].Update(msg)
			return m, cmd
		}

	// ── Confirmation ─────────────────────────────────────
	case screenConfirm:
		switch key {
		case "enter", "y":
			return m.startProcessing()
		case "esc", "b", "backspace", "n":
			m.activeScreen = screenCategories
		case "q":
			return m, tea.Quit
		}

	// ── Processing (no keyboard input accepted) ──────────
	case screenProcessing:
		// intentionally empty

	// ── Done ─────────────────────────────────────────────
	case screenDone:
		switch key {
		case "q", "esc":
			return m, tea.Quit
		case "b", "r", "enter":
			// Go back to category screen so the user can select more tools
			m.activeScreen = screenCategories
			// Clear processing state so the screen is fresh
			m.logLines = nil
			m.created = nil
			m.skipped = nil
			m.progress = 0
			m.total = 0
			m.currentTask = ""
			m.processing = false
			// Clear previous selection so they start fresh
			m.selectedTools = make(map[string]catalogue.Tool)
			return m, nil
		}
	}

	return m, nil
}

// ─────────────────────────────────────────────────────────────
//  Custom tool wizard — step advance
// ─────────────────────────────────────────────────────────────

func (m Model) advanceCustomStep() (Model, tea.Cmd) {
	m.customInputs[m.customStep].Blur()
	val := m.customInputs[m.customStep].Value()

	switch m.customStep {
	case customStepName:
		name := strings.ToLower(strings.ReplaceAll(val, " ", "-"))
		if name == "" {
			name = "custom-tool"
		}
		m.customTool.Name = name

	case customStepDisplay:
		display := val
		if display == "" {
			// Manual title-case
			words := strings.Fields(strings.ReplaceAll(m.customTool.Name, "-", " "))
			for i, w := range words {
				if len(w) > 0 {
					words[i] = strings.ToUpper(w[:1]) + w[1:]
				}
			}
			display = strings.Join(words, " ")
		}
		m.customTool.DisplayName = display

	case customStepCmd:
		m.customTool.Cmd = val

	case customStepCategory:
		if val == "" {
			m.customTool.Category = "Custom"
		} else {
			var idx int
			if _, err := fmt.Sscanf(val, "%d", &idx); err == nil &&
				idx >= 1 && idx <= len(m.categories) {
				m.customTool.Category = m.categories[idx-1]
			} else {
				m.customTool.Category = val
			}
		}

	case customStepDesc:
		m.customTool.Description = val

	case customStepPkgArch:
		m.customTool.PkgArch = val

	case customStepPkgDeb:
		m.customTool.PkgDeb = val

	case customStepSlug:
		m.customTool.KaliSlug = val
		m.customTool.IsCustom = true
		// Add to selection and return to categories
		m.selectedTools[m.customTool.Name] = m.customTool
		m.customTool = catalogue.Tool{} // reset
		for i := range m.customInputs {
			m.customInputs[i].SetValue("")
		}
		m.customStep = customStepName
		m.activeScreen = screenCategories
		return m, nil
	}

	if m.customStep < customStepSlug {
		m.customStep++
		m.customInputs[m.customStep].Focus()
	}
	return m, textinput.Blink
}

// ─────────────────────────────────────────────────────────────
//  Start processing
// ─────────────────────────────────────────────────────────────

func (m Model) startProcessing() (Model, tea.Cmd) {
	m.activeScreen = screenProcessing
	m.processing = true
	m.logLines = nil
	m.created = nil
	m.skipped = nil
	m.progress = 0

	tools := make([]catalogue.Tool, 0, len(m.selectedTools))
	for _, t := range m.selectedTools {
		tools = append(tools, t)
	}
	m.total = len(tools)

	// Start a background ticker that keeps the sudo timestamp alive so that
	// sudo commands during installs don't ask for the password again.
	// (WarmSudo was already called in main() before the TUI started.)
	if !m.noInstall && m.stopSudoKeepAlive == nil {
		m.stopSudoKeepAlive = installer.KeepSudoAlive()
	}

	state := processState{
		tools:      tools,
		noInstall:  m.noInstall,
		si:         m.sysInfo,
		terminal:   m.terminal,
		shell:      m.shell,
		desktopDir: m.desktopDir,
		iconDir:    m.iconDir,
	}

	return m, tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return processNextMsg{state: state} },
	)
}

// ─────────────────────────────────────────────────────────────
//  View dispatcher
// ─────────────────────────────────────────────────────────────

func (m Model) View() string {
	switch m.activeScreen {
	case screenWelcome:
		return m.viewWelcome()
	case screenCategories:
		return m.viewCategories()
	case screenTools:
		return m.viewTools()
	case screenCustom:
		return m.viewCustom()
	case screenConfirm:
		return m.viewConfirm()
	case screenProcessing:
		return m.viewProcessing()
	case screenDone:
		return m.viewDone()
	}
	return ""
}

// ─────────────────────────────────────────────────────────────
//  View: Welcome / Settings
// ─────────────────────────────────────────────────────────────

func (m Model) viewWelcome() string {
	var b strings.Builder

	b.WriteString(Banner() + "\n\n")

	// System info chips
	b.WriteString("  " +
		StyleBadgeCategory.Render("distro: "+string(m.sysInfo.Distro)) + "  " +
		StyleBadgeCategory.Render("de: "+m.sysInfo.Desktop) +
		"\n\n")

	b.WriteString(StyleCyan.Render("⚙  Settings") + "\n")
	b.WriteString("  " + StyleSep.Render(strings.Repeat("─", 50)) + "\n\n")

	for i, item := range m.settingsItems {
		cursor := "  "
		if i == m.settingsCursor {
			cursor = StyleCyan.Render("▶ ")
		}
		var opts []string
		for _, o := range item.options {
			if o == *item.value {
				opts = append(opts, StyleBadgeSelected.Render(" "+o+" "))
			} else {
				opts = append(opts, StyleBadgeCategory.Render(" "+o+" "))
			}
		}
		b.WriteString(fmt.Sprintf("%s%s  %s\n",
			cursor,
			StyleBold.Render(fmt.Sprintf("%-12s", item.label)),
			strings.Join(opts, " "),
		))
	}

	b.WriteString("\n")
	if m.noInstall {
		b.WriteString("  " + StyleYellow.Render("⚠  --no-install: launchers created without installing packages") + "\n")
	}

	b.WriteString("\n" + KeyBar([]KeyHint{
		{Key: "↑↓", Desc: "navigate"},
		{Key: "←→ / enter", Desc: "change option"},
		{Key: "tab / n", Desc: "continue →"},
		{Key: "q", Desc: "quit"},
	}))

	return b.String()
}

// ─────────────────────────────────────────────────────────────
//  View: Category list
// ─────────────────────────────────────────────────────────────

func (m Model) viewCategories() string {
	var b strings.Builder

	b.WriteString(Banner() + "\n\n")

	if len(m.selectedTools) > 0 {
		b.WriteString("  " + StyleBadgeSelected.Render(fmt.Sprintf(" ✓ %d selected ", len(m.selectedTools))) + "\n\n")
	} else {
		b.WriteString("  " + StyleMuted.Render("nothing selected yet") + "\n\n")
	}

	b.WriteString(StyleCyan.Render("  📁  Categories") + "\n")
	b.WriteString("  " + StyleSep.Render(strings.Repeat("─", 54)) + "\n\n")

	visH := m.height - 12
	start, end := visibleWindow(m.catCursor, len(m.categories)+1, visH)

	for i := start; i < end; i++ {
		// "Add Custom Tool" special row at the bottom of the list
		if i == len(m.categories) {
			cursor := "    "
			if m.catCursor == i {
				cursor = StylePurple.Render("  ▶ ")
			}
			b.WriteString(cursor + StylePurple.Render("✚  Add Custom Tool") + "\n")
			continue
		}

		cat := m.categories[i]
		tools := m.byCategory[cat]
		selCount := 0
		for _, t := range tools {
			if _, ok := m.selectedTools[t.Name]; ok {
				selCount++
			}
		}

		cursor := "    "
		if m.catCursor == i {
			cursor = StyleCyan.Render("  ▶ ")
		}

		tag := StyleTag.
			Background(CatColour(i)).
			Foreground(lipgloss.Color("#0d1117")).
			Render(fmt.Sprintf(" %-24s ", cat))

		count := StyleDim.Render(fmt.Sprintf("[%d]", len(tools)))

		selBadge := ""
		if selCount > 0 {
			selBadge = "  " + StyleBadgeSelected.Render(fmt.Sprintf(" %d/%d ✓ ", selCount, len(tools)))
		}

		b.WriteString(fmt.Sprintf("%s%s %s%s\n", cursor, tag, count, selBadge))
	}

	b.WriteString("\n\n" + KeyBar([]KeyHint{
		{Key: "↑↓ / jk", Desc: "navigate"},
		{Key: "enter", Desc: "open"},
		{Key: "a", Desc: "all"},
		{Key: "D", Desc: "deselect all"},
		{Key: "ctrl+p", Desc: "proceed"},
		{Key: "tab", Desc: "← settings"},
		{Key: "q", Desc: "quit"},
	}))

	return b.String()
}

// ─────────────────────────────────────────────────────────────
//  View: Tool list
// ─────────────────────────────────────────────────────────────

func (m Model) viewTools() string {
	var b strings.Builder

	catIdx := m.catCursor
	cat := m.categories[catIdx]
	tools := m.filtered
	if tools == nil {
		tools = m.byCategory[cat]
	}

	tag := StyleTag.
		Background(CatColour(catIdx)).
		Foreground(lipgloss.Color("#0d1117")).
		Render(fmt.Sprintf("  %s  ", cat))
	b.WriteString("\n  " + tag + "\n\n")

	if m.searching {
		b.WriteString("  " + StylePurple.Render("/") + " " + m.searchInput.View() + "\n\n")
	} else {
		b.WriteString("  " + StyleDim.Render("/ to search") + "\n\n")
	}

	selInCat := 0
	for _, t := range m.byCategory[cat] {
		if _, ok := m.selectedTools[t.Name]; ok {
			selInCat++
		}
	}
	b.WriteString(fmt.Sprintf("  %s  %s\n\n",
		StyleBadgeCategory.Render(fmt.Sprintf(" %d in category ", selInCat)),
		StyleBadgeSelected.Render(fmt.Sprintf(" %d total selected ", len(m.selectedTools))),
	))

	visH := m.height - 14
	start, end := visibleWindow(m.toolCursor, len(tools), visH)

	for i := start; i < end; i++ {
		t := tools[i]
		_, isSelected := m.selectedTools[t.Name]

		// Check if the tool binary is already installed on this system
		cmdBin := t.Cmd
		if idx := strings.Index(cmdBin, " "); idx >= 0 {
			cmdBin = cmdBin[:idx]
		}
		isInstalled := cmdBin != "" && system.Which(cmdBin)

		cursor := "    "
		if m.toolCursor == i {
			cursor = StyleCyan.Render("  ▶ ")
		}

		check := "  "
		var nameStr string
		if isSelected {
			check = StyleCheckmark.Render("✓ ")
			nameStr = StyleItemChecked.PaddingLeft(0).Render(t.DisplayName)
		} else if m.toolCursor == i {
			nameStr = StyleItemSelected.PaddingLeft(0).Render(t.DisplayName)
		} else {
			nameStr = StyleItemNormal.PaddingLeft(0).Render(t.DisplayName)
		}

		// "installed" badge shown in green when binary is present
		installedBadge := ""
		if isInstalled {
			installedBadge = "  " + StyleBadgeInstalled.Render("● installed")
		}

		cmdStr := StyleDim.Render("  $ " + t.Cmd)
		b.WriteString(fmt.Sprintf("%s%s%s%s%s\n", cursor, check, nameStr, installedBadge, cmdStr))
	}

	if len(tools) > visH {
		b.WriteString("\n  " + StyleDim.Render(fmt.Sprintf("%d–%d of %d", start+1, end, len(tools))) + "\n")
	}

	b.WriteString("\n" + KeyBar([]KeyHint{
		{Key: "↑↓ / jk", Desc: "navigate"},
		{Key: "space", Desc: "toggle"},
		{Key: "a", Desc: "select all"},
		{Key: "D", Desc: "deselect cat"},
		{Key: "/", Desc: "search"},
		{Key: "ctrl+p", Desc: "proceed"},
		{Key: "b / esc", Desc: "← back"},
		{Key: "q", Desc: "quit"},
	}))

	return b.String()
}

// ─────────────────────────────────────────────────────────────
//  View: Custom tool wizard
// ─────────────────────────────────────────────────────────────

func (m Model) viewCustom() string {
	var b strings.Builder

	b.WriteString("\n  " + StylePurple.Render("✚  Add Custom Tool") + "\n")
	b.WriteString("  " + StyleSep.Render(strings.Repeat("─", 52)) + "\n\n")

	labels := []string{
		"Tool ID", "Display Name", "Command", "Category",
		"Description", "Arch Package", "Deb Package", "Kali Slug",
	}
	helps := []string{
		"unique lowercase id, e.g. my-scanner",
		"human-readable name, e.g. My Scanner",
		"binary to run, e.g. myscanner -i",
		"number or name  (blank = Custom)",
		"tooltip description  (optional)",
		"pacman / AUR package name  (optional)",
		"apt package name  (optional)",
		"kali.org/tools/<slug> for icon  (optional)",
	}

	for i, label := range labels {
		var stepIcon string
		var labelStyle lipgloss.Style
		switch {
		case i < m.customStep:
			stepIcon = StyleGreen.Render("✓")
			labelStyle = StyleGreen
		case i == m.customStep:
			stepIcon = StylePurple.Render("▶")
			labelStyle = StylePurple.Copy().Bold(true)
		default:
			stepIcon = StyleDim.Render("○")
			labelStyle = StyleDim
		}

		b.WriteString(fmt.Sprintf("  %s  %s  %s\n",
			stepIcon,
			labelStyle.Render(fmt.Sprintf("%-14s", label)),
			StyleDim.Render(helps[i]),
		))
		if i == m.customStep {
			b.WriteString("     " + m.customInputs[i].View() + "\n")
		}
		b.WriteString("\n")
	}

	if m.customStep == customStepCategory {
		b.WriteString("  " + StyleDim.Render("Available categories:") + "\n")
		for i, c := range m.categories {
			b.WriteString(fmt.Sprintf("    %s  %s\n",
				StyleDim.Render(fmt.Sprintf("%2d.", i+1)),
				StyleMuted.Render(c),
			))
		}
		b.WriteString("\n")
	}

	b.WriteString(KeyBar([]KeyHint{
		{Key: "enter", Desc: "next step"},
		{Key: "shift+tab", Desc: "← back"},
		{Key: "esc", Desc: "cancel"},
	}))

	return b.String()
}

// ─────────────────────────────────────────────────────────────
//  View: Confirmation
// ─────────────────────────────────────────────────────────────

func (m Model) viewConfirm() string {
	var b strings.Builder

	b.WriteString(Banner() + "\n\n")
	b.WriteString("  " + StyleCyan.Render("🚀  Ready to create launchers") + "\n\n")
	b.WriteString("  " + StyleBold.Render(fmt.Sprintf("%d tools selected:", len(m.selectedTools))) + "\n\n")

	catGroups := map[string][]string{}
	for _, t := range m.selectedTools {
		catGroups[t.Category] = append(catGroups[t.Category], t.DisplayName)
	}
	for ci, cat := range m.categories {
		names, ok := catGroups[cat]
		if !ok {
			continue
		}
		tag := StyleTag.
			Background(CatColour(ci)).
			Foreground(lipgloss.Color("#0d1117")).
			Render(fmt.Sprintf(" %s ", cat))
		b.WriteString("  " + tag + "\n")
		for _, n := range names {
			b.WriteString("    " + StyleMuted.Render("• ") + StyleNormal.Render(n) + "\n")
		}
		b.WriteString("\n")
	}
	if names, ok := catGroups["Custom"]; ok {
		b.WriteString("  " + StylePurple.Render(" Custom ") + "\n")
		for _, n := range names {
			b.WriteString("    " + StyleMuted.Render("• ") + StyleNormal.Render(n) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("  " + StyleSep.Render(strings.Repeat("─", 52)) + "\n")
	b.WriteString(fmt.Sprintf("  Terminal  %s    Shell  %s\n",
		StyleBadgeSelected.Render(" "+m.terminal+" "),
		StyleBadgeSelected.Render(" "+m.shell+" "),
	))
	if m.noInstall {
		b.WriteString("  " + StyleYellow.Render("⚠  --no-install: packages will NOT be installed") + "\n")
	}
	b.WriteString("\n\n" + KeyBar([]KeyHint{
		{Key: "enter / y", Desc: "go"},
		{Key: "b / esc", Desc: "← back"},
		{Key: "q", Desc: "quit"},
	}))

	return b.String()
}

// ─────────────────────────────────────────────────────────────
//  View: Processing
// ─────────────────────────────────────────────────────────────

func (m Model) viewProcessing() string {
	var b strings.Builder

	b.WriteString(Banner() + "\n\n")

	b.WriteString(fmt.Sprintf("  %s  %s\n\n",
		StyleSpinner.Render(m.spinner.View()),
		StyleCyan.Render("Processing: "+m.currentTask+"…"),
	))

	b.WriteString("  " + ProgressBar(m.progress, m.total, 48) + "\n")
	b.WriteString("  " + StyleMuted.Render(fmt.Sprintf("%d / %d tools", m.progress, m.total)) + "\n\n")
	b.WriteString("  " + StyleSep.Render(strings.Repeat("─", 56)) + "\n\n")

	visH := m.height - 14
	start := 0
	if len(m.logLines) > visH {
		start = len(m.logLines) - visH
	}
	for _, ll := range m.logLines[start:] {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			ll.style.Render(ll.icon),
			ll.style.Render(ll.msg),
		))
	}

	return b.String()
}

// ─────────────────────────────────────────────────────────────
//  View: Done
// ─────────────────────────────────────────────────────────────

func (m Model) viewDone() string {
	var b strings.Builder

	b.WriteString(Banner() + "\n\n")

	if len(m.created) > 0 {
		b.WriteString("  " + StyleGreen.Render(fmt.Sprintf("✓  %d launcher(s) created successfully!", len(m.created))) + "\n\n")
	}
	if len(m.skipped) > 0 {
		b.WriteString("  " + StyleYellow.Render(fmt.Sprintf("✗  %d tool(s) skipped (install failed / command not found)", len(m.skipped))) + "\n")
		for _, s := range m.skipped {
			b.WriteString("    " + StyleMuted.Render("• ") + StyleNormal.Render(s) + "\n")
		}
		b.WriteString("\n")
	}

	home := m.sysInfo.HomeDir
	b.WriteString("  " + StyleSep.Render(strings.Repeat("─", 56)) + "\n\n")
	b.WriteString(fmt.Sprintf("  %s\n    %s\n\n",
		StyleCyan.Render("Launchers →"),
		StyleNormal.Render(filepath.Join(home, ".local/share/applications/cyber-launcher/")),
	))
	b.WriteString(fmt.Sprintf("  %s\n    %s\n\n",
		StyleCyan.Render("Icons     →"),
		StyleNormal.Render(filepath.Join(home, ".local/share/cyber-launcher/icons/")),
	))

	b.WriteString("  " + StyleBold.Render("How to use:") + "\n")
	b.WriteString("  " + StyleCyan.Render("XFCE ") + StyleNormal.Render(" → Right-click panel → Add items → Launcher") + "\n")
	b.WriteString("         " + StyleMuted.Render("Browse: ~/.local/share/applications/cyber-launcher/") + "\n")
	b.WriteString("  " + StyleCyan.Render("GNOME") + StyleNormal.Render(" → Super key → type tool name (app grid)") + "\n")
	b.WriteString("  " + StyleCyan.Render("Both ") + StyleNormal.Render(" → Drag any .desktop file to desktop or panel") + "\n\n")

	b.WriteString(KeyBar([]KeyHint{
		{Key: "b / r / enter", Desc: "← back to categories"},
		{Key: "q / esc", Desc: "quit"},
	}))

	return b.String()
}

// ─────────────────────────────────────────────────────────────
//  Single-tool processing pipeline
// ─────────────────────────────────────────────────────────────

func processSingleTool(
	t *catalogue.Tool,
	noInstall bool,
	si system.Info,
	terminal, shell, desktopDir, iconDir string,
) ([]logLine, bool) {
	var logs []logLine
	name := t.DisplayName

	// 1. Install (skip for custom tools with no package defined)
	if !noInstall && (!t.IsCustom || t.PkgArch != "" || t.PkgDeb != "") {
		logs = append(logs, logLine{"⬇", "Installing " + name + "…", StyleLogStep})
		result := installer.InstallDetailed(*t, si.Distro, si.AURHelper)
		switch {
		case result.OK && result.Stage == "already installed":
			logs = append(logs, logLine{"○", name + " — already installed", StyleLogStep})
		case result.OK && result.Stage == "primary":
			logs = append(logs, logLine{"✓", name + " — " + result.Detail, StyleLogOK})
		case result.OK && result.Stage == "alias":
			logs = append(logs, logLine{"✓", name + " — " + result.Detail, StyleLogOK})
		case result.OK && result.Stage == "ecosystem":
			logs = append(logs, logLine{"✓", name + " — " + result.Detail, StyleLogOK})
		default:
			logs = append(logs, logLine{"✗", name + " — " + result.Detail, StyleLogErr})
			return logs, false
		}
	}

	// 2. Fetch icon + description (inside WriteEntry → FetchKaliInfo)
	logs = append(logs, logLine{"🔍", "Fetching info & icon for " + name + "…", StyleLogStep})

	// 3. Write .desktop
	path, err := desktop.WriteEntry(t, terminal, shell, desktopDir, iconDir)
	if err != nil {
		logs = append(logs, logLine{"✗", fmt.Sprintf("%s — write error: %v", name, err), StyleLogErr})
		return logs, false
	}

	logs = append(logs, logLine{"✓", name + "  →  " + path, StyleLogOK})
	return logs, true
}

// ─────────────────────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────────────────────

func filterTools(tools []catalogue.Tool, query string) []catalogue.Tool {
	if query == "" {
		return tools
	}
	q := strings.ToLower(query)
	var out []catalogue.Tool
	for _, t := range tools {
		if strings.Contains(strings.ToLower(t.Name), q) ||
			strings.Contains(strings.ToLower(t.DisplayName), q) ||
			strings.Contains(strings.ToLower(t.Cmd), q) {
			out = append(out, t)
		}
	}
	return out
}

func visibleWindow(cursor, total, height int) (start, end int) {
	if height <= 0 {
		height = 20
	}
	if total <= height {
		return 0, total
	}
	start = cursor - height/2
	if start < 0 {
		start = 0
	}
	end = start + height
	if end > total {
		end = total
		start = end - height
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

func cycleOption(item *settingItem, delta int) {
	if len(item.options) == 0 {
		return
	}
	cur := 0
	for i, o := range item.options {
		if o == *item.value {
			cur = i
			break
		}
	}
	cur = (cur + delta + len(item.options)) % len(item.options)
	*item.value = item.options[cur]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ─────────────────────────────────────────────────────────────
//  Entry points called from main
// ─────────────────────────────────────────────────────────────

// Run launches the interactive Bubble Tea TUI.
func Run(noInstall bool) error {
	m := NewModel(noInstall)
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}

// RunHeadless processes tools without a TUI — used for --all / --tools flags.
// sudo credentials must have been warmed by main() before calling this.
func RunHeadless(tools []catalogue.Tool, noInstall bool) {
	si := system.Detect()
	home := si.HomeDir
	desktopDir := filepath.Join(home, ".local/share/applications/cyber-launcher")
	iconDir := filepath.Join(home, ".local/share/cyber-launcher/icons")

	if !noInstall {
		stop := installer.KeepSudoAlive()
		defer stop()
	}

	created, skipped := 0, 0
	for i := range tools {
		t := &tools[i]
		fmt.Printf("\033[1;36m[→]\033[0m  %-32s", t.DisplayName)

		logs, ok := processSingleTool(t, noInstall, si, si.Terminal, si.Shell, desktopDir, iconDir)
		// Print last log entry as the result
		if len(logs) > 0 {
			last := logs[len(logs)-1]
			if ok {
				fmt.Printf("  \033[1;32m%s\033[0m\n", last.icon)
			} else {
				fmt.Printf("  \033[1;31m✗  %s\033[0m\n", last.msg)
			}
		}

		if ok {
			created++
		} else {
			skipped++
		}
	}

	desktop.RefreshDB(home)

	fmt.Printf("\n\033[1;32m✓  Created %d launcher(s)\033[0m", created)
	if skipped > 0 {
		fmt.Printf("   \033[1;33m✗  Skipped %d\033[0m", skipped)
	}
	fmt.Println()
}

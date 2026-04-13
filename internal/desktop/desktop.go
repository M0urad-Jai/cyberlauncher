package desktop

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/cyberlauncher/cyberlauncher/internal/catalogue"
)

const (
	kaliBase  = "https://www.kali.org/tools"
	userAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:122.0) Gecko/20100101 Firefox/122.0"
)

// termExecTemplates maps terminal name → Exec= format string.
// Placeholders: {title}, {shell}, {cmd}
var termExecTemplates = map[string]string{
	"alacritty":      `alacritty -t "{title}" -e {shell} -c "{cmd} --help; exec {shell} -i"`,
	"terminator":     `terminator --title "{title}" -x {shell} -c "{cmd} --help; exec {shell} -i"`,
	"kitty":          `kitty -T "{title}" {shell} -c "{cmd} --help; exec {shell} -i"`,
	"gnome-terminal": `gnome-terminal --title "{title}" -- {shell} -c "{cmd} --help; exec {shell} -i"`,
	"xterm":          `xterm -T "{title}" -e '{shell} -c "{cmd} --help; exec {shell} -i"'`,
	"tilix":          `tilix --title "{title}" -e '{shell} -c "{cmd} --help; exec {shell} -i"'`,
	"konsole":        `konsole -p tabtitle="{title}" -e {shell} -c "{cmd} --help; exec {shell} -i"`,
	"xfce4-terminal": `xfce4-terminal --title "{title}" -x {shell} -c "{cmd} --help; exec {shell} -i"`,
	"lxterminal":     `lxterminal --title "{title}" -e '{shell} -c "{cmd} --help; exec {shell} -i"'`,
}

// fdCategories maps our category names → freedesktop categories.
var fdCategories = map[string]string{
	"Information Gathering":  "Network;Security;",
	"Vulnerability Analysis": "Security;",
	"Web Application":        "Network;Security;WebDevelopment;",
	"Exploitation":           "Security;",
	"Password Attacks":       "Security;",
	"Wireless Attacks":       "Network;Security;",
	"Sniffing & Spoofing":    "Network;Security;",
	"Post Exploitation":      "Security;",
	"Forensics":              "Security;Science;",
	"Reverse Engineering":    "Security;Development;",
	"Social Engineering":     "Security;",
	"Hardware Hacking":       "Security;Electronics;",
	"Reporting":              "Security;Office;",
	"Crypto & Stego":         "Security;",
	"Network Tools":          "Network;Security;",
	"Custom":                 "Security;",
}
 
// fallbackIconPalette — one colour per tool (by hash).
var fallbackIconPalette = []string{
	"#e63946", "#457b9d", "#2a9d8f", "#e9c46a", "#f4a261",
	"#6d6875", "#b5179e", "#4cc9f0", "#f72585", "#7209b7",
	"#3a0ca3", "#4361ee", "#06d6a0", "#ffd166", "#ef476f",
}
 
// versionRe matches common version strings in --version output.
// Examples matched: 7.4, 1.2.3, v2.0.1-beta, 2024.01.2, 3.14-rc1
var versionRe = regexp.MustCompile(`v?(\d+\.\d[\w.\-]*)`)
 
// httpClient shared for all fetches.
var httpClient = &http.Client{Timeout: 12 * time.Second}
 
// ─────────────────────────────────────────────────────────────
//  Version detection (Option A — ask the binary)
// ─────────────────────────────────────────────────────────────
 
// getBinaryVersion asks the installed binary for its version string by
// trying the most common version flags in order.  Returns the first
// version-like token found, or "" if nothing matches.
//
// The binary is run with a 3-second timeout so a hanging tool never
// blocks the .desktop generation pipeline.
func getBinaryVersion(binary string) string {
	if binary == "" {
		return ""
	}
 
	// Flags tried in order — stop at the first one that produces output
	// containing something that looks like a version number.
	flags := []string{"--version", "-V", "-v", "version", "--ver"}
 
	for _, flag := range flags {
		version := runVersionFlag(binary, flag)
		if version != "" {
			return version
		}
	}
	return ""
}
 
// runVersionFlag runs `binary flag`, captures combined output (many tools
// print to stderr), and extracts the first version-like token.
func runVersionFlag(binary, flag string) string {
	cmd := exec.Command(binary, flag) //nolint:gosec
	cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C") // avoid locale noise
 
	// Use a hard timeout via a goroutine and timer so we never block
	type result struct {
		out []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := cmd.CombinedOutput()
		ch <- result{out, err}
	}()
 
	// Wait up to 3 seconds
	select {
	case r := <-ch:
		return parseVersionToken(string(r.out))
	case <-time.After(3 * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return ""
	}
}
 
// parseVersionToken extracts the first version-like token from raw output.
// It scans all lines (some tools print "Tool vX.Y" on line 2+) and returns
// the first match, stripping a leading "v" prefix for cleanliness.
func parseVersionToken(s string) string {
	// Try to find a version on any line of the output
	for _, line := range strings.Split(s, "\n") {
		if m := versionRe.FindString(line); m != "" {
			return strings.TrimPrefix(m, "v")
		}
	}
	return ""
}
 
// ─────────────────────────────────────────────────────────────
//  Public API
// ─────────────────────────────────────────────────────────────
 
// WriteEntry creates the .desktop file for t and returns its path.
// It also populates t.Description, t.Homepage, t.IconPath via side-effects.
func WriteEntry(t *catalogue.Tool, terminal, shell, desktopDir, iconDir string) (string, error) {
	// Fetch description + homepage from kali.org if not already set
	if t.Description == "" && t.KaliSlug != "" {
		FetchKaliInfo(t)
	}
	if t.Description == "" {
		t.Description = fmt.Sprintf("%s — cybersecurity tool (%s)", t.DisplayName, t.Category)
	}
 
	// Fetch icon
	if t.IconPath == "" {
		t.IconPath = FetchIcon(t.Name, t.KaliSlug, iconDir)
	}
 
	// Detect installed version by asking the binary directly
	cmdBin := firstWord(t.Cmd)
	appVersion := getBinaryVersion(cmdBin)
 
	winTitle := windowTitle(t.DisplayName)
	execLine := buildExec(t.Cmd, terminal, shell, winTitle)
 
	fdCat := fdCategories[t.Category]
	if fdCat == "" {
		fdCat = "Security;"
	}
 
	comment := sanitiseComment(t.Description)
	iconVal := t.IconPath
	if iconVal == "" {
		iconVal = "security-medium"
	}
 
	hpLine := ""
	if t.Homepage != "" {
		hpLine = fmt.Sprintf("# Homepage: %s\n", t.Homepage)
	}
 
	// X-AppVersion is written only when a version was successfully detected.
	appVersionLine := ""
	if appVersion != "" {
		appVersionLine = fmt.Sprintf("X-AppVersion=%s\n", appVersion)
	}
 
	content := fmt.Sprintf(`[Desktop Entry]
Name=%s
Type=Application
%sGenericName=%s
Comment=%s
%sExec=%s
Icon=%s
Terminal=false
Categories=%s
Keywords=security;hacking;pentest;cybersecurity;%s;
StartupNotify=true
`, winTitle, appVersionLine, t.Category, comment, hpLine, execLine, iconVal, fdCat, t.Name)
 
	dest := filepath.Join(desktopDir, t.Name+".desktop")
	if err := os.MkdirAll(desktopDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, []byte(content), 0o755); err != nil {
		return "", err
	}
	return dest, nil
}
 
// FetchKaliInfo scrapes kali.org/tools/<slug> and fills t.Description, t.Homepage.
func FetchKaliInfo(t *catalogue.Tool) {
	if t.KaliSlug == "" {
		return
	}
	url := fmt.Sprintf("%s/%s/", kaliBase, t.KaliSlug)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")
 
	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	html := string(body)
 
	// OG description
	if m := regexp.MustCompile(`<meta\s+property="og:description"\s+content="([^"]+)"`).FindStringSubmatch(html); len(m) > 1 {
		t.Description = strings.TrimSpace(m[1])
	}
	// Fallback: first long <p>
	if t.Description == "" {
		if m := regexp.MustCompile(`<p>([^<]{50,400})</p>`).FindStringSubmatch(html); len(m) > 1 {
			t.Description = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(m[1], "")
		}
	}
 
	// Homepage link
	if m := regexp.MustCompile(`href="(https?://[^"]+)"[^>]*>\s*(?:Homepage|Source|GitHub)`).FindStringSubmatch(html); len(m) > 1 {
		t.Homepage = m[1]
	}
}
 
// FetchIcon tries to download an icon from kali.org and caches it locally.
// Falls back to an SVG with coloured initials.
func FetchIcon(toolName, kaliSlug, iconDir string) string {
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		return ""
	}
 
	// Return cached
	for _, ext := range []string{"svg", "png"} {
		p := filepath.Join(iconDir, toolName+"."+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
 
	slug := kaliSlug
	if slug == "" {
		slug = toolName
	}
	sources := []string{
		fmt.Sprintf("%s/%s/images/%s-logo.svg", kaliBase, slug, slug),
		fmt.Sprintf("%s/%s/images/%s.svg", kaliBase, slug, slug),
		fmt.Sprintf("%s/%s/images/%s-logo.png", kaliBase, slug, slug),
		fmt.Sprintf("%s/%s/images/%s.png", kaliBase, slug, slug),
		fmt.Sprintf("https://raw.githubusercontent.com/kalilinux/nethunter-store-icons/main/icons/%s.png", slug),
	}
 
	for _, src := range sources {
		if data, ok := fetchBytes(src); ok && len(data) > 512 {
			ext := "png"
			if strings.HasSuffix(src, ".svg") {
				ext = "svg"
			}
			p := filepath.Join(iconDir, toolName+"."+ext)
			if os.WriteFile(p, data, 0o644) == nil {
				return p
			}
		}
	}
 
	return writeFallbackSVG(toolName, iconDir)
}
 
// ─────────────────────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────────────────────
 
func windowTitle(displayName string) string {
	return titleCaseWords(displayName)
}
 
// WindowTitle is exported for use in the UI.
func WindowTitle(displayName string) string {
	return titleCaseWords(displayName)
}
 
func titleCaseWords(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		runes := []rune(w)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
		}
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}
 
func buildExec(cmd, terminal, shell, winTitle string) string {
	tpl, ok := termExecTemplates[terminal]
	if !ok {
		tpl = termExecTemplates["xterm"]
	}
	r := strings.NewReplacer(
		"{title}", winTitle,
		"{shell}", shell,
		"{cmd}", cmd,
	)
	return r.Replace(tpl)
}
 
func sanitiseComment(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, `"`, "'")
	if len(s) > 250 {
		s = s[:250]
	}
	return s
}
 
func fetchBytes(url string) ([]byte, bool) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return nil, false
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, err == nil
}
 
func writeFallbackSVG(toolName, iconDir string) string {
	h := 0
	for _, b := range toolName {
		h = h*31 + int(b)
	}
	if h < 0 {
		h = -h
	}
	col := fallbackIconPalette[h%len(fallbackIconPalette)]
	ini := strings.ToUpper(toolName)
	if len([]rune(ini)) > 2 {
		ini = string([]rune(ini)[:2])
	}
 
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">
  <rect width="64" height="64" rx="12" fill="%s"/>
  <text x="32" y="40" font-family="monospace" font-size="22" font-weight="bold"
        fill="white" text-anchor="middle">%s</text>
</svg>`, col, ini)
 
	p := filepath.Join(iconDir, toolName+"_fallback.svg")
	if os.WriteFile(p, []byte(svg), 0o644) == nil {
		return p
	}
	return ""
}
 
// RefreshDB updates the desktop database so new entries appear in app menus.
func RefreshDB(homeDir string) {
	appsDir := filepath.Join(homeDir, ".local/share/applications")
	for _, cmd := range [][]string{
		{"update-desktop-database", appsDir},
		{"xdg-desktop-menu", "forceupdate"},
	} {
		if _, err := exec.LookPath(cmd[0]); err == nil {
			c := exec.Command(cmd[0], cmd[1:]...) //nolint:gosec
			c.Run()                                //nolint:errcheck
		}
	}
}
 
// firstWord returns the first space-delimited token of s.
func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i]
	}
	return s
}
 
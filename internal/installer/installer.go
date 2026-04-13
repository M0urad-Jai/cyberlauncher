package installer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cyberlauncher/cyberlauncher/internal/catalogue"
	"github.com/cyberlauncher/cyberlauncher/internal/system"
)

// ─────────────────────────────────────────────────────────────
//  Sudo helpers
// ─────────────────────────────────────────────────────────────

// WarmSudo runs "sudo -v" on the real TTY before Bubble Tea takes over.
// Call once at startup so the user enters their password while the normal
// terminal is still active.  All later Install() calls reuse the timestamp.
func WarmSudo() error {
	cmd := exec.Command("sudo", "-v") //nolint:gosec
	cmd.Stdin  = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RefreshSudo silently extends the sudo credential timestamp.
func RefreshSudo() {
	_ = exec.Command("sudo", "-v", "-n").Run() //nolint:gosec
}

// KeepSudoAlive ticks every 90 s to keep the credential alive during
// long batch installs.  Call the returned stop() when done.
func KeepSudoAlive() (stop func()) {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(90 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				RefreshSudo()
			}
		}
	}()
	return func() { close(done) }
}

// ─────────────────────────────────────────────────────────────
//  Install — 3-stage fallback
// ─────────────────────────────────────────────────────────────

// InstallResult carries the outcome details so the TUI can show exactly
// which stage succeeded or which stages were attempted.
type InstallResult struct {
	OK      bool
	Stage   string // "already installed" | "primary" | "alias" | "ecosystem"
	Detail  string // human-readable reason on failure
}

// Install tries to install t in three stages:
//
//  1. Primary package name (PkgArch / PkgDeb)
//  2. Alias package names  (PkgAliasArch / PkgAliasDeb) — tried one by one
//  3. Ecosystem manager    (pip / go / cargo / gem / npm) if Ecosystem is set
//
// Stops and returns success at the first stage that works.
// Returns failure only if all applicable stages fail.
func Install(t catalogue.Tool, distro system.Distro, aurHelper string) (bool, string) {
	r := install(t, distro, aurHelper)
	return r.OK, stageDetail(r)
}

// InstallDetailed is the same as Install but returns the full result struct.
func InstallDetailed(t catalogue.Tool, distro system.Distro, aurHelper string) InstallResult {
	return install(t, distro, aurHelper)
}

func install(t catalogue.Tool, distro system.Distro, aurHelper string) InstallResult {
	cmdBin := firstWord(t.Cmd)

	// Already on the system?
	if cmdBin != "" && system.Which(cmdBin) {
		return InstallResult{OK: true, Stage: "already installed"}
	}

	if distro == system.DistroUnknown {
		return InstallResult{
			OK:     false,
			Stage:  "primary",
			Detail: "unsupported distro — cannot auto-install",
		}
	}

	// ── Stage 1: primary package ──────────────────────────
	primary := pkgForDistro(t.PkgArch, t.PkgDeb, distro)
	if primary != "" {
		if ok := tryPkgInstall(primary, distro, aurHelper); ok {
			if verifyBin(cmdBin) {
				return InstallResult{OK: true, Stage: "primary",
					Detail: fmt.Sprintf("installed via package %q", primary)}
			}
		}
	}

	// ── Stage 2: alias package names ─────────────────────
	aliases := aliasesForDistro(t.PkgAliasArch, t.PkgAliasDeb, distro)
	for _, alias := range aliases {
		if ok := tryPkgInstall(alias, distro, aurHelper); ok {
			if verifyBin(cmdBin) {
				return InstallResult{OK: true, Stage: "alias",
					Detail: fmt.Sprintf("installed via alias package %q", alias)}
			}
		}
	}

	// ── Stage 3: ecosystem package manager ───────────────
	if t.Ecosystem != catalogue.EcosystemNone && t.EcosystemPkg != "" {
		if ok := tryEcosystemInstall(t.Ecosystem, t.EcosystemPkg); ok {
			if verifyBin(cmdBin) {
				return InstallResult{OK: true, Stage: "ecosystem",
					Detail: fmt.Sprintf("installed via %s install %s", t.Ecosystem, t.EcosystemPkg)}
			}
		}
	}

	// All stages exhausted
	tried := buildTriedList(primary, aliases, t.Ecosystem, t.EcosystemPkg)
	return InstallResult{
		OK:     false,
		Stage:  "all",
		Detail: fmt.Sprintf("not found after trying: %s", tried),
	}
}

// ─────────────────────────────────────────────────────────────
//  Stage executors
// ─────────────────────────────────────────────────────────────

func tryPkgInstall(pkg string, distro system.Distro, aurHelper string) bool {
	if pkg == "" {
		return false
	}
	var args []string
	switch distro {
	case system.DistroArch:
		if aurHelper != "pacman" {
			args = []string{aurHelper, "-S", "--noconfirm", "--needed", pkg}
		} else {
			args = []string{"sudo", "pacman", "-S", "--noconfirm", "--needed", pkg}
		}
	case system.DistroDebian:
		runWithTTY([]string{"sudo", "apt", "update", "-qq"},
			[]string{"DEBIAN_FRONTEND=noninteractive"})
		args = []string{"sudo", "apt", "install", "-y",
			"-o", "Dpkg::Options::=--force-confnew", pkg}
	default:
		return false
	}
	_, err := runWithTTY(args, []string{"DEBIAN_FRONTEND=noninteractive"})
	return err == nil
}

func tryEcosystemInstall(eco catalogue.Ecosystem, pkg string) bool {
	if pkg == "" {
		return false
	}
	var args []string
	switch eco {
	case catalogue.EcosystemPip:
		// Try pip3 first, fall back to pip
		if system.Which("pip3") {
			args = []string{"pip3", "install", "--user", pkg}
		} else if system.Which("pip") {
			args = []string{"pip", "install", "--user", pkg}
		} else {
			return false
		}
	case catalogue.EcosystemGo:
		if !system.Which("go") {
			return false
		}
		args = []string{"go", "install", pkg}
	case catalogue.EcosystemCargo:
		if !system.Which("cargo") {
			return false
		}
		args = []string{"cargo", "install", pkg}
	case catalogue.EcosystemGem:
		if !system.Which("gem") {
			return false
		}
		args = []string{"gem", "install", "--user-install", pkg}
	case catalogue.EcosystemNpm:
		if !system.Which("npm") {
			return false
		}
		args = []string{"npm", "install", "-g", pkg}
	default:
		return false
	}
	_, err := runWithTTY(args, nil)
	return err == nil
}

// ─────────────────────────────────────────────────────────────
//  Internal helpers
// ─────────────────────────────────────────────────────────────

func pkgForDistro(arch, deb string, distro system.Distro) string {
	switch distro {
	case system.DistroArch:
		return arch
	case system.DistroDebian:
		return deb
	}
	return ""
}

func aliasesForDistro(arch, deb []string, distro system.Distro) []string {
	switch distro {
	case system.DistroArch:
		return arch
	case system.DistroDebian:
		return deb
	}
	return nil
}

func verifyBin(cmdBin string) bool {
	if cmdBin == "" {
		return true // nothing to verify
	}
	return system.Which(cmdBin)
}

func buildTriedList(primary string, aliases []string, eco catalogue.Ecosystem, ecoPkg string) string {
	var parts []string
	if primary != "" {
		parts = append(parts, fmt.Sprintf("pkg:%s", primary))
	}
	for _, a := range aliases {
		parts = append(parts, fmt.Sprintf("alias:%s", a))
	}
	if eco != catalogue.EcosystemNone && ecoPkg != "" {
		parts = append(parts, fmt.Sprintf("%s:%s", eco, ecoPkg))
	}
	if len(parts) == 0 {
		return "no install method defined"
	}
	return strings.Join(parts, ", ")
}

func stageDetail(r InstallResult) string {
	if r.OK {
		return r.Stage
	}
	return r.Detail
}

// runWithTTY executes args with:
//   stdin  → /dev/tty (sudo password prompt visible)
//   stdout → /dev/null (package manager output hidden from TUI)
//   stderr → /dev/null (same)
func runWithTTY(args []string, extraEnv []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("empty command")
	}

	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}

	// Discard output so nothing leaks into the Bubble Tea alt-screen
	if devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0); err == nil {
		cmd.Stdout = devNull
		cmd.Stderr = devNull
		defer devNull.Close()
	}

	// Attach stdin to /dev/tty so sudo can prompt for a password
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		cmd.Stdin = tty
		defer tty.Close()
	}

	return "", cmd.Run()
}

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i]
	}
	return s
}

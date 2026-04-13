package system

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Distro represents the detected Linux distribution family.
type Distro string

const (
	DistroArch    Distro = "arch"
	DistroDebian  Distro = "debian"
	DistroUnknown Distro = "unknown"
)

// Info holds everything detected about the current system environment.
type Info struct {
	Distro     Distro
	AURHelper  string // paru / yay / pacman
	Desktop    string // gnome / xfce / kde / generic
	Terminal   string // alacritty / terminator / …
	Shell      string // zsh / bash / fish / sh
	HomeDir    string
}

// Detect probes the running system and returns an Info struct.
func Detect() Info {
	info := Info{
		HomeDir: homeDir(),
	}
	info.Distro = detectDistro()
	info.AURHelper = detectAUR()
	info.Desktop = detectDesktop()
	info.Terminal = detectTerminal()
	info.Shell = detectShell()
	return info
}

// ─────────────────────────────────────────────────────────────

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("HOME")
}

func detectDistro() Distro {
	f, err := os.Open("/etc/os-release")
	if err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.ToLower(sc.Text())
			for _, arch := range []string{"arch", "manjaro", "endeavouros", "garuda", "artix"} {
				if strings.Contains(line, arch) {
					return DistroArch
				}
			}
			for _, deb := range []string{"debian", "ubuntu", "kali", "parrot", "linuxmint", "raspbian", "pop"} {
				if strings.Contains(line, deb) {
					return DistroDebian
				}
			}
		}
	}
	if which("pacman") { return DistroArch }
	if which("apt")    { return DistroDebian }
	return DistroUnknown
}

func detectAUR() string {
	for _, h := range []string{"paru", "yay", "trizen", "pikaur"} {
		if which(h) {
			return h
		}
	}
	return "pacman"
}

func detectDesktop() string {
	de := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP"))
	switch {
	case strings.Contains(de, "xfce"):  return "xfce"
	case strings.Contains(de, "gnome"): return "gnome"
	case strings.Contains(de, "kde"):   return "kde"
	case strings.Contains(de, "lxde"):  return "lxde"
	case strings.Contains(de, "mate"):  return "mate"
	default: return "generic"
	}
}

func detectTerminal() string {
	preferred := []string{
		"alacritty", "terminator", "kitty", "tilix",
		"gnome-terminal", "xfce4-terminal", "konsole",
		"lxterminal", "xterm",
	}
	for _, t := range preferred {
		if which(t) {
			return t
		}
	}
	return "xterm"
}

func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "bash"
	}
	return filepath.Base(shell)
}

// which returns true if the binary exists in PATH.
func which(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// Which is the exported variant used by other packages.
func Which(bin string) bool {
	return which(bin)
}

// KnownTerminals returns all supported terminal names in preference order.
func KnownTerminals() []string {
	return []string{
		"alacritty", "terminator", "kitty", "tilix",
		"gnome-terminal", "xfce4-terminal", "konsole",
		"lxterminal", "xterm",
	}
}

// KnownShells returns all supported shell names.
func KnownShells() []string {
	return []string{"zsh", "bash", "fish", "sh"}
}

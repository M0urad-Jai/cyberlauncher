package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cyberlauncher/cyberlauncher/internal/catalogue"
	"github.com/cyberlauncher/cyberlauncher/internal/installer"
	"github.com/cyberlauncher/cyberlauncher/internal/ui"
)

const version = "0.0.1"

func main() {
	// ── Flags ──────────────────────────────────────────────
	var (
		fAll       = flag.Bool("all", false, "Process ALL tools in catalogue")
		fTools     = flag.String("tools", "", "Comma-separated list of tool names to process")
		fNoInstall = flag.Bool("no-install", false, "Skip package installation")
		fList      = flag.Bool("list", false, "List all available tools and exit")
		fVersion   = flag.Bool("version", false, "Print version and exit")
	)
	flag.Usage = usage
	flag.Parse()

	if *fVersion {
		fmt.Println("CyberLauncher v" + version)
		os.Exit(0)
	}

	if *fList {
		printList()
		os.Exit(0)
	}

	// ── Prime sudo credentials BEFORE the TUI / processing starts.
	//
	//    Bubble Tea switches the terminal to alt-screen mode, which
	//    detaches stdin from the real TTY.  Any sudo call that needs
	//    a password after that point will silently fail because there
	//    is no TTY available for the password prompt.
	//
	//    WarmSudo() runs "sudo -v" right now — on the normal terminal —
	//    so the user can type their password once.  All subsequent
	//    sudo calls use --non-interactive and rely on the cached
	//    credential timestamp (default: 15 min).
	//
	//    If --no-install is set we skip this entirely.
	if !*fNoInstall {
		if err := installer.WarmSudo(); err != nil {
			fmt.Fprintf(os.Stderr,
				"\033[1;31m[✗]\033[0m  sudo authentication failed: %v\n"+
					"    Use --no-install to skip package installation.\n", err)
			os.Exit(1)
		}
		// Keep the credential timestamp alive for long runs.
		stopSudo := installer.KeepSudoAlive()
		defer stopSudo()
	}

	// ── Headless paths (--all / --tools) ───────────────────
	if *fAll {
		printBanner()
		all := catalogue.AllTools()
		fmt.Printf("Processing all %d tools…\n\n", len(all))
		ui.RunHeadless(all, *fNoInstall)
		return
	}

	if *fTools != "" {
		byName := map[string]catalogue.Tool{}
		for _, t := range catalogue.AllTools() {
			byName[t.Name] = t
		}
		var picked []catalogue.Tool
		for _, name := range strings.Split(*fTools, ",") {
			name = strings.TrimSpace(strings.ToLower(name))
			if t, ok := byName[name]; ok {
				picked = append(picked, t)
			} else {
				fmt.Printf("\033[1;33m[!]\033[0m  '%s' not found in catalogue — skipping\n", name)
			}
		}
		if len(picked) == 0 {
			fmt.Println("\033[1;31m[✗]\033[0m  No valid tools specified.")
			os.Exit(1)
		}
		printBanner()
		ui.RunHeadless(picked, *fNoInstall)
		return
	}

	// ── Interactive TUI ────────────────────────────────────
	if err := ui.Run(*fNoInstall); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}


// ─────────────────────────────────────────────────────────────

func printBanner() {
	const c = "\033[1;36m"
	const w = "\033[1;37m"
	const d = "\033[2m"
	const r = "\033[0m"
	fmt.Printf(`
%s╔══════════════════════════════════════════════════════════════╗
║  %s⚡  CyberLauncher%s v%s — Kali Tools Desktop Launcher Generator  %s║
║  %sXFCE / GNOME  •  Arch / Debian  •  Any shell / terminal%s      ║
╚══════════════════════════════════════════════════════════════╝%s

`, c, w, c, version, c, d, c, r)
}

func printList() {
	printBanner()
	cats := catalogue.Categories()
	byCat := catalogue.ByCategory()
	total := 0
	for _, cat := range cats {
		tools := byCat[cat]
		fmt.Printf("\033[1;33m%s\033[0m  \033[2m(%d)\033[0m\n", cat, len(tools))
		for _, t := range tools {
			fmt.Printf("  \033[1;32m•\033[0m %-28s  \033[2m%s\033[0m\n", t.Name, t.Cmd)
		}
		fmt.Println()
		total += len(tools)
	}
	fmt.Printf("\033[1;37mTotal: \033[1;32m%d\033[1;37m tools across \033[1;32m%d\033[1;37m categories\033[0m\n\n", total, len(cats))
}

func usage() {
	printBanner()
	fmt.Print(`Usage:
  cyberlauncher [flags]

Flags:
  --all            Process ALL tools in catalogue (headless mode)
  --tools=a,b,c   Specific tool names, comma-separated (headless mode)
  --no-install     Skip package installation (create launchers only)
  --list           List all available tools and exit
  --version        Print version and exit

Interactive mode (default, no flags):
  Launches the Bubble Tea TUI for guided tool selection.

Examples:
  cyberlauncher                                 # interactive TUI
  cyberlauncher --tools=nmap,sqlmap,hydra       # headless, specific tools
  cyberlauncher --all --no-install              # all launchers, skip installs
  cyberlauncher --list                          # catalogue overview

`)
}

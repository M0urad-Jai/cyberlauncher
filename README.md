# ⚡ CyberLauncher

> A beautiful Bubble Tea TUI for generating `.desktop` launcher entries for 200+
> Kali Linux security tools — on **Arch/Debian**, **XFCE/GNOME**, with any
> **terminal emulator** and **shell**.

```
╔══════════════════════════════════════════════════════════════╗
║  ⚡  CyberLauncher — Kali Tools Desktop Entry Generator  ⚡  ║
║  XFCE / GNOME  •  Arch / Debian  •  Any shell / terminal     ║
╚══════════════════════════════════════════════════════════════╝
```

---

## Features

- **200+ tools** across 15 categories (nmap, metasploit, burpsuite, hashcat, wireshark, ghidra…)
- **Beautiful Bubble Tea TUI** — category browser, tool picker, live search, progress log
- **Auto-detection** of distro, AUR helper, desktop environment, terminal, shell
- **Package installation** via `pacman`/`yay`/`paru` or `apt` — skips if already present
- **Icon fetching** from `kali.org/tools/<slug>` with coloured-initials SVG fallback
- **Description scraping** from kali.org tool pages
- **Custom tool wizard** — add any tool not in the catalogue
- **No launcher on failure** — if install fails or binary isn't found, no `.desktop` is created
- **Exec line** format: `alacritty -t "Nmap" -e zsh -c "nmap --help; exec zsh -i"`
- **Window title** = tool display name with every word capitalised
- **freedesktop-compliant** `.desktop` entries, compatible with XFCE, GNOME, KDE, etc.

---

## Requirements

- **Go 1.22+**
- Linux with a freedesktop-compliant desktop environment

---

## Quick Start

```bash
# Clone
git clone https://github.com/cyberlauncher/cyberlauncher
cd cyberlauncher

# Build & install to ~/.local/bin/
chmod +x setup.sh && ./setup.sh

# Launch the TUI
cyberlauncher
```

Or build and run directly:
```bash
make run
```

---

## Usage

```
cyberlauncher [flags]
```

| Flag | Description |
|------|-------------|
| _(none)_ | Launch the interactive Bubble Tea TUI |
| `--all` | Process all 200+ tools (headless) |
| `--tools=a,b,c` | Specific tools by name, comma-separated |
| `--no-install` | Skip package installation, create launchers only |
| `--list` | Print all tools grouped by category and exit |
| `--version` | Print version |

### Examples

```bash
cyberlauncher                           # interactive TUI
cyberlauncher --tools=nmap,sqlmap,hydra # headless, three tools
cyberlauncher --all --no-install        # all launchers, no installs
cyberlauncher --list                    # catalogue overview
```

---

## TUI Navigation

| Key | Action |
|-----|--------|
| `↑↓` / `j k` | Move cursor |
| `←→` / `h l` | Change setting option |
| `enter` / `space` | Select/toggle / confirm |
| `a` | Select all in current scope |
| `D` | Deselect all in current scope |
| `/` | Search tools (tool screen) |
| `esc` / `b` | Go back one level |
| `tab` | Switch between Welcome ↔ Categories |
| `ctrl+p` | Proceed to confirmation |
| `q` / `ctrl+c` | Quit |

---

## What gets created

For each tool a `.desktop` file is written to:
```
~/.local/share/applications/cyber-launcher/<toolname>.desktop
```

Icons are cached in:
```
~/.local/share/cyber-launcher/icons/
```

### Example `.desktop` entry (nmap)

```ini
[Desktop Entry]
X-AppVersion=7.99
Type=Application
Name=Nmap
GenericName=Information Gathering
Comment=Nmap is a utility for network exploration or security auditing.
Exec=alacritty -t "Nmap" -e zsh -c "nmap --help; exec zsh -i"
Icon=/home/user/.local/share/cyber-launcher/icons/nmap.svg
Terminal=false
Categories=Network;Security;
Keywords=security;hacking;pentest;cybersecurity;nmap;
StartupNotify=true
```

### Exec line pattern

```
<terminal> -t "<Title>" -e <shell> -c "<cmd>; exec <shell> -i"
```

The window title is the tool's display name with every word capitalised.

---

## Supported Terminals

| Terminal | Title flag |
|----------|-----------|
| alacritty | `-t "Title"` |
| terminator | `--title "Title"` |
| kitty | `-T "Title"` |
| gnome-terminal | `--title "Title"` |
| xterm | `-T "Title"` |
| tilix | `--title "Title"` |
| konsole | `-p tabtitle="Title"` |
| xfce4-terminal | `--title "Title"` |
| lxterminal | `--title "Title"` |

---

## Project Structure

```
cyberlauncher/
├── main.go                         # CLI entry point
├── go.mod
├── Makefile
└── internal/
    ├── catalogue/catalogue.go      # 200+ tool definitions
    ├── system/system.go            # distro/DE/terminal/shell detection
    ├── installer/installer.go      # pacman/apt installer
    ├── desktop/desktop.go          # .desktop writer, icon fetcher, kali scraper
    └── ui/
        ├── styles.go               # lipgloss theme & styles
        └── model.go                # Bubble Tea model (all screens)
```

---

## Categories

| Category | Tools |
|----------|-------|
| Information Gathering | nmap, masscan, rustscan, amass, subfinder, theHarvester, recon-ng… |
| Vulnerability Analysis | nikto, sqlmap, nuclei, openvas, wpscan, lynis… |
| Web Application | burpsuite, zaproxy, gobuster, ffuf, feroxbuster, dalfox… |
| Exploitation | metasploit, msfvenom, exploitdb, routersploit, beef-xss… |
| Password Attacks | hashcat, john, hydra, medusa, crunch, cewl, ncrack… |
| Wireless Attacks | aircrack-ng, kismet, wifite, reaver, pixiewps, bully… |
| Sniffing & Spoofing | wireshark, bettercap, responder, mitmproxy, ettercap… |
| Post Exploitation | metasploit, bloodhound, evil-winrm, impacket, netexec… |
| Forensics | autopsy, volatility3, binwalk, exiftool, bulk-extractor… |
| Reverse Engineering | ghidra, radare2, cutter, jadx, apktool, yara, strace… |
| Social Engineering | gophish, evilginx2, king-phisher… |
| Hardware Hacking | hackrf, rtl-sdr, gnuradio, openocd, flashrom… |
| Crypto & Stego | steghide, sslscan, testssl, hashcat, xortool… |
| Network Tools | netcat, socat, proxychains4, chisel, ligolo-ng, scapy… |
| Reporting | faraday, dradis, pipal, metagoofil… |

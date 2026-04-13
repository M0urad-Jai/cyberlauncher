#!/usr/bin/env bash
# ╔══════════════════════════════════════════════════════════════╗
# ║  CyberLauncher — setup.sh                                   ║
# ║  Run this once after unzipping to build and install.        ║
# ╚══════════════════════════════════════════════════════════════╝
set -euo pipefail

C='\033[1;36m'; G='\033[1;32m'; Y='\033[1;33m'; R='\033[1;31m'; W='\033[1;37m'; D='\033[0m'

info()  { echo -e "${G}[+]${D} $*"; }
warn()  { echo -e "${Y}[!]${D} $*"; }
err()   { echo -e "${R}[✗]${D} $*" >&2; exit 1; }
step()  { echo -e "${C}[→]${D} $*"; }

echo -e "${C}"
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  ⚡  CyberLauncher — Build & Install                        ║"
echo "╚══════════════════════════════════════════════════════════════╝${D}"
echo

# ── 1. Check Go ───────────────────────────────────────────────
if ! command -v go &>/dev/null; then
    err "Go is not installed. Install Go 1.21+ from https://go.dev/dl/ then re-run this script."
fi

GO_VERSION=$(go version | grep -oP '\d+\.\d+' | head -1)
GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)

if [[ "$GO_MAJOR" -lt 1 ]] || { [[ "$GO_MAJOR" -eq 1 ]] && [[ "$GO_MINOR" -lt 21 ]]; }; then
    err "Go $GO_VERSION is too old. Need Go 1.21+. Download from https://go.dev/dl/"
fi
info "Go $GO_VERSION found ✓"

# ── 2. Move into the project directory ───────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"
[[ -f "go.mod" ]] || err "go.mod not found — run this script from the cyberlauncher/ directory."

# ── 3. go mod tidy  (downloads deps + generates go.sum) ──────
step "Running go mod tidy (downloads Bubble Tea, lipgloss, bubbles)…"
go mod tidy
info "Dependencies resolved ✓"

# ── 4. Build ──────────────────────────────────────────────────
step "Building cyberlauncher…"
mkdir -p dist
go build -ldflags "-s -w" -o dist/cyberlauncher .
info "Built dist/cyberlauncher ✓"

# ── 5. Install to ~/.local/bin ────────────────────────────────
INSTALL_DIR="$HOME/.local/bin"
mkdir -p "$INSTALL_DIR"
cp dist/cyberlauncher "$INSTALL_DIR/cyberlauncher"
chmod +x "$INSTALL_DIR/cyberlauncher"
info "Installed to $INSTALL_DIR/cyberlauncher ✓"

# ── 6. PATH check ─────────────────────────────────────────────
if ! echo "$PATH" | grep -q "$HOME/.local/bin"; then
    warn "~/.local/bin is not in your PATH."
    echo
    echo "  Add one of these lines to your shell config:"
    echo -e "  ${C}bash/zsh${D}  →  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc"
    echo -e "  ${C}fish${D}     →  fish_add_path ~/.local/bin"
    echo
fi

echo
echo -e "${G}╔══════════════════════════════════════════════════════╗${D}"
echo -e "${G}║  ✓  CyberLauncher installed successfully!           ║${D}"
echo -e "${G}╚══════════════════════════════════════════════════════╝${D}"
echo
echo -e "  Run:  ${C}cyberlauncher${D}               (interactive TUI)"
echo -e "  Run:  ${C}cyberlauncher --list${D}         (list all 200 tools)"
echo -e "  Run:  ${C}cyberlauncher --help${D}         (all options)"
echo

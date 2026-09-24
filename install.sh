#!/bin/bash
set -euo pipefail

REPO="${MULTIGRAVITY_REPO:-yegear1/multigravity-cli}"
BRANCH="${MULTIGRAVITY_BRANCH:-main}"
RAW="https://raw.githubusercontent.com/$REPO/$BRANCH"
INSTALL_DIR="/usr/local/bin"

# ── helpers ──────────────────────────────────────────────────────────────────
print_step () { echo "  → $1"; }
abort ()       { echo "Error: $1" >&2; exit 1; }

# ── platform & architecture ──────────────────────────────────────────────────
case "$(uname -s)" in
  Darwin)
    PLATFORM="darwin"
    ;;
  Linux)
    PLATFORM="linux"
    ;;
  *)
    abort "unsupported platform. Multigravity currently supports macOS and Linux."
    ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)
    ARCH="amd64"
    ;;
  arm64|aarch64)
    ARCH="arm64"
    ;;
  *)
    abort "unsupported architecture: $ARCH"
    ;;
esac

# ── preflight ────────────────────────────────────────────────────────────────
command -v curl &>/dev/null || abort "curl is required but not found"

# fall back to ~/.local/bin if /usr/local/bin isn't writable without sudo
if [ ! -w "$INSTALL_DIR" ]; then
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"

  # auto-add to PATH in the user's shell profile if not already there
  if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    case "${SHELL:-}" in
      */zsh)  SHELL_RC="$HOME/.zshrc" ;;
      */fish) SHELL_RC="$HOME/.config/fish/config.fish" ;;
      *)      SHELL_RC="$HOME/.bashrc" ;;
    esac

    LINE='export PATH="$HOME/.local/bin:$PATH"'

    if [ "$SHELL_RC" = "$HOME/.config/fish/config.fish" ]; then
      LINE='fish_add_path "$HOME/.local/bin"'
    fi

    if ! grep -qF "$HOME/.local/bin" "$SHELL_RC" 2>/dev/null; then
      echo "" >> "$SHELL_RC"
      echo "# Added by Multigravity installer" >> "$SHELL_RC"
      echo "$LINE" >> "$SHELL_RC"
      print_step "Added $INSTALL_DIR to PATH in $SHELL_RC"
    fi

    # apply to current session so the next commands in this script work
    export PATH="$INSTALL_DIR:$PATH"
  fi
fi

echo "Installing Multigravity to $INSTALL_DIR ..."

# ── install binary or fallback script ────────────────────────────────────────
INSTALLED=0

# Option A: Build from local source if inside repository and Go toolchain is available
if [ -f "./cmd/multigravity/main.go" ] && command -v go &>/dev/null; then
  print_step "Building binary from local source with Go..."
  if go build -o "$INSTALL_DIR/multigravity" ./cmd/multigravity; then
    chmod +x "$INSTALL_DIR/multigravity"
    INSTALLED=1
  fi
fi

# Option B: Download pre-compiled release binary
if [ "$INSTALLED" -eq 0 ]; then
  ASSET_NAME="multigravity-$PLATFORM-$ARCH"
  RELEASE_URL="https://github.com/$REPO/releases/latest/download/$ASSET_NAME"
  print_step "Downloading pre-compiled binary ($ASSET_NAME)..."
  if curl -fsSL "$RELEASE_URL" -o "$INSTALL_DIR/multigravity" 2>/dev/null; then
    chmod +x "$INSTALL_DIR/multigravity"
    INSTALLED=1
  fi
fi

# Option C: Fallback to standalone script
if [ "$INSTALLED" -eq 0 ]; then
  print_step "Release asset unavailable; falling back to standalone script..."
  if curl -fsSL "$RAW/legacy/multigravity" -o "$INSTALL_DIR/multigravity" 2>/dev/null || \
     curl -fsSL "$RAW/multigravity" -o "$INSTALL_DIR/multigravity"; then
    chmod +x "$INSTALL_DIR/multigravity"
    INSTALLED=1
  else
    abort "failed to install multigravity"
  fi
fi

# ── download macOS icon ──────────────────────────────────────────────────────
if [ "$PLATFORM" = "darwin" ]; then
  print_step "Downloading icon..."
  curl -fsSL "$RAW/icon.icns" -o "$INSTALL_DIR/icon.icns"
fi

echo ""
echo "✓ Multigravity installed successfully!"
echo ""
echo "Reload your shell to apply PATH changes:"
echo "  source ~/.zshrc   (or ~/.bashrc, or open a new terminal)"
echo ""
echo "Usage:"
echo "  multigravity help"
echo "  multigravity new <profile-name>"
echo "  multigravity <profile-name>"

if [ "$PLATFORM" = "linux" ] && ! command -v antigravity &>/dev/null && ! command -v agy &>/dev/null && [ ! -x /usr/share/antigravity/antigravity ]; then
  echo ""
  echo "Note:"
  echo "  Antigravity (or 'agy') was not found on this machine."
  echo "  Install Antigravity/agy for Linux and ensure 'antigravity' or 'agy' is on PATH,"
  echo "  or launch Multigravity with MULTIGRAVITY_APP=/path/to/antigravity."
fi

#!/bin/sh
# Install pre-commit hook (cross-platform via git itself)
# 对应 ROADMAP Round 6 / F1

set -e

HOOK_SRC="scripts/git/hooks/pre-commit"
HOOK_DEST=".git/hooks/pre-commit"

if [ ! -f "$HOOK_SRC" ]; then
    echo "ERROR: $HOOK_SRC not found"
    exit 1
fi

# Use git config to install the hook (works on all platforms)
mkdir -p .git/hooks
git config core.hooksPath .git/hooks 2>/dev/null || true

# Copy via shell (cp) or powershell fallback
if command -v cp >/dev/null 2>&1; then
    cp "$HOOK_SRC" "$HOOK_DEST"
else
    powershell -Command "Copy-Item '$HOOK_SRC' '$HOOK_DEST' -Force"
fi

# Make executable (chmod) or use git
if command -v chmod >/dev/null 2>&1; then
    chmod +x "$HOOK_DEST"
fi

echo "pre-commit hook installed to $HOOK_DEST"

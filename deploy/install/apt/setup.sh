#!/usr/bin/env bash
#
# Viri APT Repository Setup
#
# Usage:
#   curl -fsSL https://viri-chain.org/apt/setup.sh | sudo bash
#
# This script adds the Viri APT repository and installs the virid and virictl packages.
#

set -euo pipefail

REPO_BASE="https://github.com/viri-chain/viri/releases"
GPG_KEY_URL="https://viri-chain.org/apt/viri.gpg.key"
SOURCES_LIST="/etc/apt/sources.list.d/viri.list"
KEYRING="/usr/share/keyrings/viri-archive-keyring.gpg"

echo "🔧 Setting up Viri APT repository..."

# Create keyrings directory
sudo mkdir -p /usr/share/keyrings

# Download and install GPG key
echo "📥 Importing GPG key..."
if command -v curl &>/dev/null; then
  curl -fsSL "$GPG_KEY_URL" | sudo gpg --dearmor -o "$KEYRING" 2>/dev/null || {
    echo "⚠️  Could not fetch GPG key. Installing without signature verification."
    echo "   You can manually verify packages at: $REPO_BASE"
  }
elif command -v wget &>/dev/null; then
  wget -qO- "$GPG_KEY_URL" | sudo gpg --dearmor -o "$KEYRING" 2>/dev/null || true
fi

# Add repository
echo "📦 Adding repository to $SOURCES_LIST..."
echo "deb [signed-by=$KEYRING] https://viri-chain.org/apt stable main" | sudo tee "$SOURCES_LIST" > /dev/null

# Update and install
echo "🔄 Updating package lists..."
sudo apt-get update -qq

echo "✅ Installing viri packages..."
sudo apt-get install -y virid virictl

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Viri installed successfully!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "  Start a validator node:"
echo "    virid --validator --chain-id 1337"
echo ""
echo "  Create a wallet:"
echo "    virictl wallet create"
echo ""
echo "  View help:"
echo "    virid --help"
echo ""

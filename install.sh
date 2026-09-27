#!/usr/bin/env bash
# ==============================================================================
# Knowledge Faultorance (FeaziestFlow) - Windows Backend Installer
# Usage: ./install.sh [PORT]
# Example: ./install.sh 5005
# ==============================================================================

set -e

PORT="${1:-5005}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$SCRIPT_DIR/scripts/install-service.ps1" -Port "$PORT"

#!/usr/bin/env bash
# ==============================================================================
# Knowledge Faultorance (FeaziestFlow) - Windows Backend Uninstaller
# Usage: ./uninstall.sh
# ==============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$SCRIPT_DIR/scripts/uninstall-service.ps1"

#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ "${1:-}" == "--uninstall" ]]; then
    echo "Uninstalling..."
    # Add uninstall logic here
    echo "Done."
    exit 0
fi

echo "Installing..."
# Add install logic here
echo "Done."

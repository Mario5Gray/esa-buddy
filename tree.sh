#!/usr/bin/env bash
# tree.sh - project tree, filtering out noise.
# Usage: ./tree.sh [path]

tree -a -I '.git|node_modules|vendor|__pycache__|.venv|.idea|.vscode|dist|build' "${1:-.}"

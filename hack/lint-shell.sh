#!/usr/bin/env bash
# Lint repository shell scripts with ShellCheck.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v shellcheck >/dev/null 2>&1; then
  echo "shellcheck not found; install ShellCheck or run: pre-commit run shellcheck --all-files" >&2
  exit 1
fi

mapfile -t scripts < <(find "${ROOT}/hack" -name '*.sh' -type f | sort)
if [[ -f "${ROOT}/.devcontainer/post-install.sh" ]]; then
  scripts+=("${ROOT}/.devcontainer/post-install.sh")
fi

shellcheck --severity=warning "${scripts[@]}"

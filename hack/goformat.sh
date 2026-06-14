#!/usr/bin/env bash
# Apply gofmt, goimports, and golines to project Go sources.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

mapfile -t dirs < <(go list -f '{{.Dir}}' ./... 2>/dev/null || true)
if ((${#dirs[@]} == 0)); then
	echo "format: no Go packages"
	exit 0
fi

for dir in "${dirs[@]}"; do
	gofmt -w "${dir}"/*.go
	go tool goimports -local github.com/conduit-ops/mkurator -w "${dir}"/*.go
	for f in "${dir}"/*.go; do
		if [[ "$f" == */api/v1alpha1/*_types.go ]]; then
			continue
		fi
		go tool golines -w --max-len=120 --shorten-comments "$f"
	done
done

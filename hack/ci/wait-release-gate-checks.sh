#!/usr/bin/env bash
# Poll GitHub check-runs on a commit until required workflows are success.
# Used by .github/workflows/release-gate.yaml (external CI / integration / e2e).
set -euo pipefail

SHA="${1:?usage: wait-release-gate-checks.sh <full-commit-sha>}"
REPO="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY required}"
TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
POLL_TIMEOUT_MINUTES="${POLL_TIMEOUT_MINUTES:-120}"
POLL_INTERVAL_SECONDS="${POLL_INTERVAL_SECONDS:-60}"

if [[ -z "${TOKEN}" ]]; then
	echo "error: GH_TOKEN or GITHUB_TOKEN required for check-runs API" >&2
	exit 1
fi

# Job names from ci.yaml, integration.yaml, e2e.yaml (jobs.<id>.name).
REQUIRED_CHECKS=(
	gitleaks
	verify
	lint
	test
	build
	docker-build
	helm-lint
	integration
	"e2e (kustomize)"
)

deadline=$(( $(date +%s) + POLL_TIMEOUT_MINUTES * 60 ))

fetch_check_runs_json() {
	gh api \
		-H "Accept: application/vnd.github+json" \
		"/repos/${REPO}/commits/${SHA}/check-runs?per_page=100" \
		--jq '[.check_runs[] | select(.head_sha == "'"${SHA}"'") | {name, status, conclusion, html_url, started_at}]'
}

latest_by_name() {
	local runs="$1"
	local name status conclusion
	for name in "${REQUIRED_CHECKS[@]}"; do
		local line
		line="$(echo "${runs}" | jq -c --arg n "${name}" '
			map(select(.name == $n)) | sort_by(.started_at) | last // empty
		')"
		if [[ -z "${line}" || "${line}" == "null" ]]; then
			printf '%s\tmissing\t\t\t\n' "${name}"
			continue
		fi
		status="$(echo "${line}" | jq -r '.status')"
		conclusion="$(echo "${line}" | jq -r '.conclusion // ""')"
		printf '%s\t%s\t%s\t%s\n' "${name}" "${status}" "${conclusion}" "$(echo "${line}" | jq -r '.html_url // ""')"
	done
}

print_manual_checklist() {
	cat >&2 <<EOF

Release gate: external checks not green on ${SHA}

Before tagging vX.Y.Z, confirm CI, Integration, and E2E (kustomize) succeeded on this exact SHA:

  gh run list --workflow ci.yaml --commit ${SHA} --limit 5
  gh run list --workflow integration.yaml --commit ${SHA} --limit 5
  gh run list --workflow e2e.yaml --commit ${SHA} --limit 5
  gh run view <run-id> --json headSha,conclusion,status,url

If workflows were skipped (paths-ignore), re-run on this commit:

  gh workflow run integration.yaml --ref main
  gh workflow run e2e.yaml --ref main

Then re-dispatch Release gate with sha=${SHA} (or wait for poll timeout).

See docs/RELEASE.md#automated-release-gate-workflow

EOF
}

all_required_success() {
	local runs="$1"
	local line name status conclusion
	local missing=0 failed=0 pending=0
	while IFS=$'\t' read -r name status conclusion _url; do
		if [[ "${status}" == "missing" ]]; then
			missing=$((missing + 1))
			echo "  missing: ${name}" >&2
			continue
		fi
		if [[ "${status}" != "completed" ]]; then
			pending=$((pending + 1))
			echo "  pending: ${name} (${status})" >&2
			continue
		fi
		if [[ "${conclusion}" != "success" ]]; then
			failed=$((failed + 1))
			echo "  not success: ${name} (${conclusion})" >&2
			continue
		fi
		echo "  ok: ${name}"
	done < <(latest_by_name "${runs}")

	if [[ "${missing}" -gt 0 || "${failed}" -gt 0 ]]; then
		return 1
	fi
	if [[ "${pending}" -gt 0 ]]; then
		return 2
	fi
	return 0
}

export GH_TOKEN="${TOKEN}"

echo "Polling check-runs on ${REPO}@${SHA} (timeout ${POLL_TIMEOUT_MINUTES}m, interval ${POLL_INTERVAL_SECONDS}s)"

while true; do
	now=$(date +%s)
	if [[ "${now}" -ge "${deadline}" ]]; then
		echo "error: timed out waiting for required check-runs" >&2
		runs="$(fetch_check_runs_json)"
		latest_by_name "${runs}" | column -t -s $'\t' >&2 || true
		print_manual_checklist
		exit 1
	fi

	runs="$(fetch_check_runs_json)"
	set +e
	all_required_success "${runs}"
	rc=$?
	set -e

	if [[ "${rc}" -eq 0 ]]; then
		echo "All required check-runs succeeded on ${SHA}."
		exit 0
	fi

	if [[ "${rc}" -eq 1 ]]; then
		echo "error: required check-runs failed or missing on ${SHA}" >&2
		latest_by_name "${runs}" | column -t -s $'\t' >&2 || true
		print_manual_checklist
		exit 1
	fi

	echo "Waiting for pending check-runs (${POLL_INTERVAL_SECONDS}s)..."
	sleep "${POLL_INTERVAL_SECONDS}"
done

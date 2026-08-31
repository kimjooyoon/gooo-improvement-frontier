#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(pwd)"
run_id="${GITHUB_RUN_ID:-local-$(date -u +%Y%m%dT%H%M%SZ)}"
output_root="${CI_OUTPUT_ROOT:-${RUNNER_TEMP:-/tmp}/gooo-improvement-frontier-${run_id}}"
mkdir -p "$output_root/logs" "$output_root/conformance-a" "$output_root/conformance-b"
attempts="$output_root/failed-attempts.ndjson"
: > "$attempts"

phase() {
  local name="$1"
  shift
  local log="$output_root/logs/${name}.log"
  local started ended status
  started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  set +e
  "$@" >"$log" 2>&1
  status=$?
  set -e
  ended="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  jq -cn --arg phase "$name" --arg status "$([ "$status" -eq 0 ] && echo SUCCESS || echo FAILED)" --arg started "$started" --arg ended "$ended" --arg log "$log" --argjson exit_code "$status" '{phase:$phase,status:$status,started_at:$started,ended_at:$ended,exit_code:$exit_code,log:$log}' >> "$attempts"
  cat "$log"
  if [ "$status" -ne 0 ]; then
    return "$status"
  fi
}

build_time="$output_root/build.time"
test_time="$output_root/test.time"
test_json="$output_root/go-test.json"
binary="$output_root/gooo-improvement-frontier"
ir_a="$output_root/semantic-ir.json"
generated_go_a="$output_root/semantic.gooo.go"
ir_b="$output_root/semantic-ir-b.json"
generated_go_b="$output_root/semantic.gooo-b.go"

phase format bash -c 'find . -type f -name "*.go" -not -path "./.git/*" -print0 | xargs -0 -r gofmt -l | tee "$1"; test ! -s "$1"' _ "$output_root/gofmt.txt"
phase build /usr/bin/time -f '%e %M' -o "$build_time" go build -o "$binary" ./cmd/gooo-improvement-frontier
phase test /usr/bin/time -f '%e %M' -o "$test_time" go test -json -count=1 ./... > "$test_json"
phase vet go vet ./...
phase compile-a "$binary" compile --source examples/improvement-frontier.gooo --contract contracts/improvement-frontier-denominator-v1.json --output-ir "$ir_a" --output-go "$generated_go_a"
phase generated-ir cmp "$ir_a" internal/generated/semantic-ir.json
phase generated-go cmp "$generated_go_a" internal/generated/semantic.gooo.go
phase conformance-a "$binary" conformance --root "$repo_root" --source examples/improvement-frontier.gooo --contract contracts/improvement-frontier-denominator-v1.json --corpus examples/canonical-corpus.json --output-dir "$output_root/conformance-a"
phase compile-b "$binary" compile --source examples/improvement-frontier.gooo --contract contracts/improvement-frontier-denominator-v1.json --output-ir "$ir_b" --output-go "$generated_go_b"
phase deterministic-ir cmp "$ir_a" "$ir_b"
phase deterministic-go cmp "$generated_go_a" "$generated_go_b"
phase conformance-b "$binary" conformance --root "$repo_root" --source examples/improvement-frontier.gooo --contract contracts/improvement-frontier-denominator-v1.json --corpus examples/canonical-corpus.json --output-dir "$output_root/conformance-b"
phase deterministic-plan diff -ru "$output_root/conformance-a" "$output_root/conformance-b"

read -r build_seconds build_rss < "$build_time"
read -r test_seconds test_rss < "$test_time"
build_wall_ms="$(awk -v value="$build_seconds" 'BEGIN { printf "%d", (value * 1000) + 0.5 }')"
test_wall_ms="$(awk -v value="$test_seconds" 'BEGIN { printf "%d", (value * 1000) + 0.5 }')"
peak_rss_kib="$(awk -v build="$build_rss" -v test="$test_rss" 'BEGIN { if (build > test) print build; else print test }')"
tests_discovered="$(jq -s '[.[] | select(.Action == "run" and .Test != null)] | length' "$test_json")"
tests_executed="$(jq -s '[.[] | select((.Action == "pass" or .Action == "fail") and .Test != null)] | length' "$test_json")"
tests_skipped="$(jq -s '[.[] | select(.Action == "skip" and .Test != null)] | length' "$test_json")"
tests_reused="$(jq -s '[.[] | select(.Action == "output" and .Test != null and ((.Output // "") | contains("(cached)")))] | length' "$test_json")"
tests_not_observed="$(awk -v discovered="$tests_discovered" -v executed="$tests_executed" -v skipped="$tests_skipped" 'BEGIN { value = discovered - executed - skipped; if (value < 0) value = 0; print value }')"
valid_evidence="$(find "$output_root/conformance-a" -name plan.json -print0 | xargs -0 -r jq -s '[.[] | .evidence[] | select(.valid == true)] | length')"
operation_cases="$(find "$output_root/conformance-a" -name plan.json -print0 | xargs -0 -r jq -s '[.[] | .case_id] | length')"
ci_job_id="$(gh api "repos/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}/jobs" --paginate --jq '.jobs[] | select(.name == "conformance") | .id' 2>/dev/null | tail -n 1 || true)"
if [ -z "$ci_job_id" ]; then ci_job_id="${GITHUB_JOB:-conformance}"; fi

file_count="$(find . -type f -not -path './.git/*' -not -path './README.md' | wc -l | tr -d ' ')"
directory_count="$(find . -type d -not -path './.git' -not -path './.git/*' | wc -l | tr -d ' ')"
physical_lines="$(find . -type f -not -path './.git/*' -not -path './README.md' -print0 | xargs -0 -r awk '{ total++ } END { print total + 0 }')"
go_files="$(find . -type f -name '*.go' -not -path './.git/*' | wc -l | tr -d ' ')"
go_lines="$(find . -type f -name '*.go' -not -path './.git/*' -print0 | xargs -0 -r awk '{ total++ } END { print total + 0 }')"
gooo_lines="$(find . -type f -name '*.gooo' -not -path './.git/*' -print0 | xargs -0 -r awk '{ total++ } END { print total + 0 }')"

jq -n \
  --arg schema 'gooo/improvement-frontier/ci-runtime/v1' \
  --arg run_id "$run_id" \
  --arg job_id "$ci_job_id" \
  --argjson build_wall_ms "$build_wall_ms" \
  --argjson test_wall_ms "$test_wall_ms" \
  --argjson peak_rss_kib "$peak_rss_kib" \
  --argjson tests_discovered "$tests_discovered" \
  --argjson tests_executed "$tests_executed" \
  --argjson tests_reused "$tests_reused" \
  --argjson tests_skipped "$tests_skipped" \
  --argjson tests_not_observed "$tests_not_observed" \
  --argjson directories "$directory_count" \
  --argjson files "$file_count" \
  --argjson physical_lines "$physical_lines" \
  --argjson go_files "$go_files" \
  --argjson go_lines "$go_lines" \
  --argjson gooo_lines "$gooo_lines" \
  --argjson valid_evidence "$valid_evidence" \
  --argjson operation_cases "$operation_cases" \
  '{schema:$schema,ci_run_id:$run_id,ci_job_id:$job_id,build_wall_ms:$build_wall_ms,test_wall_ms:$test_wall_ms,peak_rss_kib:$peak_rss_kib,tests:{discovered:$tests_discovered,executed:$tests_executed,reused:$tests_reused,skipped:$tests_skipped,not_observed:$tests_not_observed},operations:{executed:0,reused:$valid_evidence,skipped:0,not_observed:0,canonical_cases:$operation_cases},inventory:{directories:$directories,files:$files,physical_lines:$physical_lines,go_files:$go_files,go_lines:$go_lines,gooo_lines:$gooo_lines,root_readme_excluded:true},product_authority:{repository_writes:0,local_test_executions:0,cross_project_required_gates:0},development_actions:["checkout","format_check","build","test","vet","compile","conformance","determinism_check"]}' > "$output_root/runtime-receipt.json"

if [ -n "$(git status --porcelain --untracked-files=all)" ]; then
  git status --porcelain --untracked-files=all >&2
  echo 'repository changed during CI conformance' >&2
  exit 1
fi

printf 'CI_OUTPUT_ROOT=%s\n' "$output_root"


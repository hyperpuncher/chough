#!/usr/bin/env bash
# Compare cold CLI and warm server performance with hyperfine.
#
# Default:
#   ./benchmarks/benchmark.sh
#
# Compatible positional form:
#   ./benchmarks/benchmark.sh <audio-file> [baseline-binary] [candidate-binary]
#
# Explicit form:
#   ./benchmarks/benchmark.sh --baseline ./old-chough --candidate ./dist/chough

set -euo pipefail

usage() {
	cat <<'EOF'
Usage: benchmark.sh [options] [audio-file [baseline-binary [candidate-binary]]]

Options:
  --baseline PATH   baseline chough binary
  --candidate PATH  candidate chough binary (default: ./dist/chough)
  --fixture PATH    audio fixture; may be repeated
  --runs N          measured runs after one warmup (default: 3)
  --cold-only       skip warm server benchmarks
  --warm-only       skip cold CLI benchmarks
  --output DIR      result directory (default: /tmp/chough-benchmarks/<timestamp>)
  -h, --help        show this help

Without fixture arguments, the 15-second and 1-minute fixtures are used.
EOF
}

require_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "error: required command not found: $1" >&2
		exit 1
	fi
}

shell_quote() {
	printf '%q' "$1"
}

CANDIDATE_BINARY="${CANDIDATE_BINARY:-./dist/chough}"
BASELINE_BINARY="${BASELINE_BINARY:-}"
CANDIDATE_NAME="${CANDIDATE_NAME:-candidate}"
BASELINE_NAME="${BASELINE_NAME:-baseline}"
RUNS="${RUNS:-3}"
CANDIDATE_THREADS="${CANDIDATE_THREADS:-4}"
BASELINE_THREADS="${BASELINE_THREADS:-4}"
RUN_COLD=true
RUN_WARM=true
RESULTS_DIR="${RESULTS_DIR:-}"
FIXTURES=()
POSITIONAL=()

while (($# > 0)); do
	case "$1" in
	--baseline)
		BASELINE_BINARY="${2:?--baseline requires a path}"
		shift 2
		;;
	--candidate)
		CANDIDATE_BINARY="${2:?--candidate requires a path}"
		shift 2
		;;
	--fixture)
		FIXTURES+=("${2:?--fixture requires a path}")
		shift 2
		;;
	--runs)
		RUNS="${2:?--runs requires a number}"
		shift 2
		;;
	--cold-only)
		RUN_WARM=false
		shift
		;;
	--warm-only)
		RUN_COLD=false
		shift
		;;
	--output)
		RESULTS_DIR="${2:?--output requires a directory}"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	--*)
		echo "error: unknown option: $1" >&2
		usage >&2
		exit 1
		;;
	*)
		POSITIONAL+=("$1")
		shift
		;;
	esac
done

if ((${#POSITIONAL[@]} > 3)); then
	echo "error: too many positional arguments" >&2
	exit 1
fi
if ((${#POSITIONAL[@]} > 0)); then
	FIXTURES+=("${POSITIONAL[0]}")
fi
if ((${#POSITIONAL[@]} > 1)) && [[ -z "$BASELINE_BINARY" ]]; then
	BASELINE_BINARY="${POSITIONAL[1]}"
fi
if ((${#POSITIONAL[@]} > 2)); then
	CANDIDATE_BINARY="${POSITIONAL[2]}"
fi
if ((${#FIXTURES[@]} == 0)); then
	FIXTURES=("test/audio/audio-15sec.wav" "test/audio/audio-1min.wav")
fi
if [[ ! "$RUNS" =~ ^[1-9][0-9]*$ ]]; then
	echo "error: --runs must be a positive integer" >&2
	exit 1
fi
if [[ ! "$BASELINE_THREADS" =~ ^[1-9][0-9]*$ || ! "$CANDIDATE_THREADS" =~ ^[1-9][0-9]*$ ]]; then
	echo "error: BASELINE_THREADS and CANDIDATE_THREADS must be positive integers" >&2
	exit 1
fi
if [[ "$RUN_COLD" != true && "$RUN_WARM" != true ]]; then
	echo "error: at least one benchmark mode is required" >&2
	exit 1
fi

require_command hyperfine
require_command ffprobe
require_command jq
if [[ "$RUN_WARM" == true ]]; then
	require_command curl
fi

BINARY_NAMES=("$CANDIDATE_NAME")
BINARIES=("$CANDIDATE_BINARY")
BINARY_THREADS=("$CANDIDATE_THREADS")
if [[ -n "$BASELINE_BINARY" ]]; then
	BINARY_NAMES=("$BASELINE_NAME" "$CANDIDATE_NAME")
	BINARIES=("$BASELINE_BINARY" "$CANDIDATE_BINARY")
	BINARY_THREADS=("$BASELINE_THREADS" "$CANDIDATE_THREADS")
fi
for binary in "${BINARIES[@]}"; do
	if [[ ! -x "$binary" ]]; then
		echo "error: binary is not executable: $binary" >&2
		exit 1
	fi
done
for fixture in "${FIXTURES[@]}"; do
	if [[ ! -f "$fixture" ]]; then
		echo "error: fixture not found: $fixture" >&2
		exit 1
	fi
done

TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
if [[ -z "$RESULTS_DIR" ]]; then
	RESULTS_DIR="/tmp/chough-benchmarks/$TIMESTAMP"
fi
mkdir -p "$RESULTS_DIR/raw"
RESULTS_NDJSON="$RESULTS_DIR/results.ndjson"
: >"$RESULTS_NDJSON"

fixture_duration() {
	ffprobe -v error -show_entries format=duration -of csv=p=0 "$1"
}

append_hyperfine_results() {
	local mode="$1"
	local fixture="$2"
	local duration="$3"
	local raw_json="$4"
	local startup_seconds="${5:-}"
	local server_peak_bytes="${6:-}"
	local processing_seconds="${7:-}"
	local threads="${8:-4}"

	jq -c \
		--arg mode "$mode" \
		--arg fixture "$fixture" \
		--argjson duration "$duration" \
		--arg startup "$startup_seconds" \
		--arg server_peak "$server_peak_bytes" \
		--arg processing "$processing_seconds" \
		--argjson threads "$threads" '
		.results[] | {
			mode: $mode,
			fixture: $fixture,
			audio_duration_seconds: $duration,
			implementation: .command,
			sherpa_threads: $threads,
			mean_seconds: .mean,
			stddev_seconds: .stddev,
			min_seconds: .min,
			max_seconds: .max,
			user_seconds: .user,
			system_seconds: .system,
			times_seconds: .times,
			peak_memory_bytes: (
				if $mode == "warm" and $server_peak != "" then ($server_peak | tonumber)
				elif (.memory_usage_byte // [] | length) > 0 then (.memory_usage_byte | max)
				else null end
			),
			client_peak_memory_bytes: (
				if $mode == "warm" and (.memory_usage_byte // [] | length) > 0 then (.memory_usage_byte | max)
				else null end
			),
			startup_seconds: (if $startup == "" then null else ($startup | tonumber) end),
			processing_seconds: (if $processing == "" then null else ($processing | tonumber) end)
		}' "$raw_json" >>"$RESULTS_NDJSON"
}

run_cold_benchmarks() {
	local fixture duration raw_json command i

	for fixture in "${FIXTURES[@]}"; do
		duration=$(fixture_duration "$fixture")
		for i in "${!BINARIES[@]}"; do
			raw_json="$RESULTS_DIR/raw/cold_${i}_$(basename "$fixture").json"
			command="$(shell_quote "${BINARIES[$i]}") -f text $(shell_quote "$fixture")"
			echo "cold: ${BINARY_NAMES[$i]} / $(basename "$fixture")"
			# Keep each binary in a separate hyperfine process. On Linux,
			# memory_usage_byte is a cumulative child-process high-water mark.
			hyperfine --warmup 1 --runs "$RUNS" --export-json "$raw_json" --command-name "${BINARY_NAMES[$i]}" "$command"
			append_hyperfine_results "cold" "$fixture" "$duration" "$raw_json" "" "" "" "${BINARY_THREADS[$i]}"
		done
	done
}

SERVER_PID=""
SERVER_LOG=""
STARTUP_SECONDS=""
cleanup_server() {
	if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
		kill -TERM "$SERVER_PID" 2>/dev/null || true
		wait "$SERVER_PID" 2>/dev/null || true
	fi
	SERVER_PID=""
}
trap cleanup_server EXIT INT TERM

start_server() {
	local binary="$1"
	local name="$2"
	local port="$3"
	local started_ns ready_ns

	SERVER_LOG="$RESULTS_DIR/raw/server_${name}.log"
	started_ns=$(date +%s%N)
	"$binary" --server --host 127.0.0.1 --port "$port" --workers 1 > /dev/null 2>"$SERVER_LOG" &
	SERVER_PID=$!
	for _ in {1..1200}; do
		if curl --fail --silent --output /dev/null "http://127.0.0.1:$port/health"; then
			ready_ns=$(date +%s%N)
			STARTUP_SECONDS=$(awk -v start="$started_ns" -v ready="$ready_ns" 'BEGIN { printf "%.6f", (ready - start) / 1000000000 }')
			return 0
		fi
		if ! kill -0 "$SERVER_PID" 2>/dev/null; then
			wait "$SERVER_PID" 2>/dev/null || true
			echo "error: $name server exited before readiness" >&2
			cat "$SERVER_LOG" >&2
			return 1
		fi
		sleep 0.1
	done
	echo "error: timed out waiting for $name server" >&2
	cat "$SERVER_LOG" >&2
	return 1
}

server_peak_bytes() {
	local pid="$1"
	if [[ -r "/proc/$pid/status" ]]; then
		awk '/^VmHWM:/ { print $2 * 1024; found=1 } END { if (!found) print "" }' "/proc/$pid/status"
	fi
}

run_warm_benchmarks() {
	local i binary name port startup fixture duration raw_json response processing peak
	local command
	port=$((20000 + ($$ % 20000)))

	for i in "${!BINARIES[@]}"; do
		binary="${BINARIES[$i]}"
		name="${BINARY_NAMES[$i]}"
		echo "warm server: $name"
		start_server "$binary" "$name" "$port"
		startup="$STARTUP_SECONDS"

		for fixture in "${FIXTURES[@]}"; do
			duration=$(fixture_duration "$fixture")
			raw_json="$RESULTS_DIR/raw/warm_${name}_$(basename "$fixture").json"
			command="curl --fail --silent --output /dev/null -F $(shell_quote "file=@$fixture") -F format=json http://127.0.0.1:$port/transcribe"
			echo "warm: $name / $(basename "$fixture")"
			hyperfine --warmup 1 --runs "$RUNS" --export-json "$raw_json" --command-name "$name" "$command"

			response="$RESULTS_DIR/raw/response_${name}_$(basename "$fixture").json"
			curl --fail --silent --output "$response" -F "file=@$fixture" -F format=json "http://127.0.0.1:$port/transcribe"
			processing=$(jq -r '.processing_time_seconds // empty' "$response")
			peak=$(server_peak_bytes "$SERVER_PID")
			append_hyperfine_results "warm" "$fixture" "$duration" "$raw_json" "$startup" "$peak" "$processing" "${BINARY_THREADS[$i]}"
		done

		cleanup_server
		port=$((port + 1))
	done
}

if [[ "$RUN_COLD" == true ]]; then
	run_cold_benchmarks
fi
if [[ "$RUN_WARM" == true ]]; then
	run_warm_benchmarks
fi

binary_metadata='[]'
for i in "${!BINARIES[@]}"; do
	checksum=""
	if command -v sha256sum >/dev/null 2>&1; then
		checksum=$(sha256sum "${BINARIES[$i]}" | awk '{print $1}')
	fi
	binary_metadata=$(jq -c \
		--arg name "${BINARY_NAMES[$i]}" \
		--arg path "${BINARIES[$i]}" \
		--arg sha256 "$checksum" \
		--argjson threads "${BINARY_THREADS[$i]}" \
		'. + [{name: $name, path: $path, sha256: (if $sha256 == "" then null else $sha256 end), sherpa_threads: $threads}]' <<<"$binary_metadata")
done

cpu=$(lscpu 2>/dev/null | awk -F: '/Model name/{sub(/^[[:space:]]+/, "", $2); print $2; exit}' || true)
governor=$(cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor 2>/dev/null || true)
metadata=$(jq -n -c \
	--arg generated_at "$TIMESTAMP" \
	--arg host "$(hostname)" \
	--arg os "$(uname -srm)" \
	--arg cpu "$cpu" \
	--arg governor "$governor" \
	--arg model "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8" \
	--argjson runs "$RUNS" \
	--argjson binaries "$binary_metadata" \
	'{generated_at: $generated_at, host: $host, os: $os, cpu: $cpu, cpu_governor: (if $governor == "" then null else $governor end), model: $model, warmups: 1, runs: $runs, binaries: $binaries}')

JSON_FILE="$RESULTS_DIR/benchmark.json"
jq -s --argjson metadata "$metadata" '{metadata: $metadata, results: .}' "$RESULTS_NDJSON" >"$JSON_FILE"
mv "$RESULTS_NDJSON" "$RESULTS_DIR/raw/results.ndjson"

MARKDOWN_FILE="$RESULTS_DIR/benchmark.md"
{
	echo "# benchmark results"
	echo
	echo "- generated: $TIMESTAMP"
	echo "- host: $(hostname)"
	echo "- cpu: ${cpu:-unknown}"
	echo "- model: sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8"
	echo "- runs: $RUNS after 1 warmup"
	echo
	echo "| mode | fixture | binary | threads | mean | stddev | realtime | peak rss | startup | processing |"
	echo "| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |"
	jq -r '
		def seconds: if . == null then "-" else ((. * 1000 | round) / 1000 | tostring) + "s" end;
		def memory: if . == null then "-" else ((. / 1048576 | round) | tostring) + "MB" end;
		.results[] |
		"| \(.mode) | \(.fixture | split("/")[-1]) | \(.implementation) | \(.sherpa_threads) | \(.mean_seconds | seconds) | \(.stddev_seconds | seconds) | \((.audio_duration_seconds / .mean_seconds * 10 | round) / 10)x | \(.peak_memory_bytes | memory) | \(.startup_seconds | seconds) | \(.processing_seconds | seconds) |"' "$JSON_FILE"

	if [[ -n "$BASELINE_BINARY" ]]; then
		echo
		echo "## candidate change"
		echo
		echo "| mode | fixture | latency change |"
		echo "| --- | --- | ---: |"
		for mode in cold warm; do
			for fixture in "${FIXTURES[@]}"; do
				baseline_mean=$(jq -r --arg mode "$mode" --arg fixture "$fixture" --arg name "$BASELINE_NAME" '.results[] | select(.mode == $mode and .fixture == $fixture and .implementation == $name) | .mean_seconds' "$JSON_FILE")
				candidate_mean=$(jq -r --arg mode "$mode" --arg fixture "$fixture" --arg name "$CANDIDATE_NAME" '.results[] | select(.mode == $mode and .fixture == $fixture and .implementation == $name) | .mean_seconds' "$JSON_FILE")
				if [[ -n "$baseline_mean" && -n "$candidate_mean" ]]; then
					change=$(awk -v baseline="$baseline_mean" -v candidate="$candidate_mean" 'BEGIN { printf "%+.1f%%", (candidate / baseline - 1) * 100 }')
					echo "| $mode | $(basename "$fixture") | $change |"
				fi
			done
		done
	fi

	echo
	echo "warm peak rss is measured from the server process; hyperfine client memory remains available in JSON."
} >"$MARKDOWN_FILE"

if command -v bunx >/dev/null 2>&1; then
	bunx oxfmt "$MARKDOWN_FILE" >/dev/null 2>&1 || true
fi

cat "$MARKDOWN_FILE"
echo
echo "json: $JSON_FILE"
echo "markdown: $MARKDOWN_FILE"

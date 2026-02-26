#!/usr/bin/env bash
set -euo pipefail

binary_path="${1:?binary path required}"
os_name="${2:?os required}"
arch="${3:?arch required}"

if [[ "$os_name" == "linux" ]]; then
	# shellcheck disable=SC2016 # literal runtime loader token required by patchelf
	patchelf --set-rpath '$ORIGIN' "$binary_path"
	exit 0
fi

if [[ "$os_name" != "darwin" ]]; then
	exit 0
fi

libs_dir="/tmp/chough-libbundle/${os_name}_${arch}"
if [[ ! -d "$libs_dir" ]]; then
	echo "missing libs dir: $libs_dir" >&2
	exit 1
fi

install_name_tool_cmd=""
for candidate in install_name_tool llvm-install-name-tool-18 llvm-install-name-tool arm64-apple-darwin25-install_name_tool; do
	if command -v "$candidate" >/dev/null 2>&1; then
		install_name_tool_cmd="$candidate"
		break
	fi
done

if [[ -z "$install_name_tool_cmd" ]]; then
	echo "install_name_tool not found" >&2
	exit 1
fi

dylibs=()
for path in "$libs_dir"/*.dylib; do
	if [[ -f "$path" ]]; then
		dylibs+=("$(basename "$path")")
	fi
done

# Ensure runtime loader can resolve sidecar dylibs next to executable.
"$install_name_tool_cmd" -add_rpath "@loader_path" "$binary_path" 2>/dev/null || true

for lib in "${dylibs[@]}"; do
	"$install_name_tool_cmd" -change "@rpath/${lib}" "@loader_path/${lib}" "$binary_path" 2>/dev/null || true
done

for lib in "${dylibs[@]}"; do
	"$install_name_tool_cmd" -id "@loader_path/${lib}" "${libs_dir}/${lib}" 2>/dev/null || true
	for dep in "${dylibs[@]}"; do
		"$install_name_tool_cmd" -change "@rpath/${dep}" "@loader_path/${dep}" "${libs_dir}/${lib}" 2>/dev/null || true
	done
done

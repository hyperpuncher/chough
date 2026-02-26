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

dylibs=()
for path in "$libs_dir"/*.dylib; do
	if [[ -f "$path" ]]; then
		dylibs+=("$(basename "$path")")
	fi
done

for lib in "${dylibs[@]}"; do
	install_name_tool -change "@rpath/${lib}" "@loader_path/${lib}" "$binary_path" 2>/dev/null || true
done

for lib in "${dylibs[@]}"; do
	for dep in "${dylibs[@]}"; do
		install_name_tool -change "@rpath/${dep}" "@loader_path/${dep}" "${libs_dir}/${lib}" 2>/dev/null || true
	done
done

#!/usr/bin/env bash
set -euo pipefail

mod_cache="$(go env GOMODCACHE)"
if command -v cygpath >/dev/null 2>&1; then
	mod_cache="$(cygpath -u "$mod_cache")"
fi

linux_version="$(go list -m -f '{{.Version}}' github.com/k2-fsa/sherpa-onnx-go-linux)"
macos_version="$(go list -m -f '{{.Version}}' github.com/k2-fsa/sherpa-onnx-go-macos)"
windows_version="$(go list -m -f '{{.Version}}' github.com/k2-fsa/sherpa-onnx-go-windows)"

bundle_root="/tmp/chough-libbundle"

mkdir -p "$bundle_root/linux_amd64"
mkdir -p "$bundle_root/darwin_amd64"
mkdir -p "$bundle_root/darwin_arm64"
mkdir -p "$bundle_root/windows_amd64"

chmod -R u+w "$bundle_root" 2>/dev/null || true

cp -f "${mod_cache}/github.com/k2-fsa/sherpa-onnx-go-linux@${linux_version}/lib/x86_64-unknown-linux-gnu"/*.so "$bundle_root/linux_amd64/"
cp -f "${mod_cache}/github.com/k2-fsa/sherpa-onnx-go-macos@${macos_version}/lib/x86_64-apple-darwin"/*.dylib "$bundle_root/darwin_amd64/"
cp -f "${mod_cache}/github.com/k2-fsa/sherpa-onnx-go-macos@${macos_version}/lib/aarch64-apple-darwin"/*.dylib "$bundle_root/darwin_arm64/"

cp -f "${mod_cache}/github.com/k2-fsa/sherpa-onnx-go-windows@${windows_version}/lib/x86_64-pc-windows-gnu/sherpa-onnx-c-api.dll" "$bundle_root/windows_amd64/"
cp -f "${mod_cache}/github.com/k2-fsa/sherpa-onnx-go-windows@${windows_version}/lib/x86_64-pc-windows-gnu/onnxruntime.dll" "$bundle_root/windows_amd64/"
cp -f "${mod_cache}/github.com/k2-fsa/sherpa-onnx-go-windows@${windows_version}/lib/x86_64-pc-windows-gnu/sherpa-onnx-cxx-api.dll" "$bundle_root/windows_amd64/"

mkdir -p .tmp-libbundle
ln -sfn "$bundle_root" .tmp-libbundle/current

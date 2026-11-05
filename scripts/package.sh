#!/usr/bin/env bash
# Builds titties for the current runner and packages it with the native libraries it needs.
# usage: scripts/package.sh <target> <version>   target: linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64
set -euo pipefail

target=$1
version=$2
name="titties-${version}-${target}"
out="dist/${name}"

case "$target" in
linux-amd64) triple=x86_64-unknown-linux-gnu os=linux ;;
linux-arm64) triple=aarch64-unknown-linux-gnu os=linux ;;
darwin-amd64) triple=x86_64-apple-darwin os=macos ;;
darwin-arm64) triple=aarch64-apple-darwin os=macos ;;
windows-amd64) triple=x86_64-pc-windows-gnu os=windows ;;
*) echo "unknown target $target" >&2 && exit 1 ;;
esac

rm -rf "$out"
mkdir -p "$out"

exe=titties
[ "$os" = windows ] && exe=titties.exe
CGO_ENABLED=1 go build -trimpath -ldflags "-s -w -X main.version=${version}" -o "$out/$exe" .

sherpa=$(go list -m -f '{{.Dir}}' "github.com/k2-fsa/sherpa-onnx-go-${os}/v2")/lib/${triple}
phonemize=$(go list -m -f '{{.Dir}}' "github.com/csukuangfj/piper-phonemize-go-${os}")/lib/${triple}

case "$os" in
linux)
	mkdir -p "$out/lib"
	cp "$sherpa/libsherpa-onnx-c-api.so" "$sherpa/libonnxruntime.so" "$phonemize/libpiper_phonemize_core.so" "$out/lib/"
	# Replace the rpath into the Go module cache of the build machine with the lib dir next to the binary.
	# shellcheck disable=SC2016 # $ORIGIN must reach patchelf literally
	patchelf --set-rpath '$ORIGIN/lib' "$out/$exe"
	readelf -d "$out/$exe" | grep -E 'NEEDED|RUNPATH'
	;;
macos)
	mkdir -p "$out/lib"
	cp "$sherpa"/libsherpa-onnx-c-api.dylib "$sherpa"/libonnxruntime*.dylib "$phonemize/libpiper_phonemize_core.dylib" "$out/lib/"
	chmod u+w "$out"/lib/*
	# Drop the rpaths into the Go module cache and look next to the binary instead.
	for rp in $(otool -l "$out/$exe" | awk '/cmd LC_RPATH/{getline; getline; print $2}'); do
		install_name_tool -delete_rpath "$rp" "$out/$exe"
	done
	install_name_tool -add_rpath @executable_path/lib "$out/$exe"
	# Make every reference to a bundled library go through @rpath.
	for lib in "$out"/lib/*.dylib; do
		base=$(basename "$lib")
		install_name_tool -id "@rpath/$base" "$lib"
		for f in "$out/$exe" "$out"/lib/*.dylib; do
			old=$(otool -L "$f" | awk -v b="$base" '$1 ~ ("/" b "$") {print $1}' | head -1)
			if [ -n "$old" ] && [ "$old" != "@rpath/$base" ]; then
				install_name_tool -change "$old" "@rpath/$base" "$f"
			fi
		done
		install_name_tool -add_rpath @loader_path "$lib" 2>/dev/null || true
	done
	# install_name_tool invalidates signatures: sign again, ad hoc, or Apple Silicon refuses to run them.
	codesign --force --sign - "$out"/lib/*.dylib "$out/$exe"
	otool -L "$out/$exe"
	;;
windows)
	# Windows looks for DLLs next to the executable.
	cp "$sherpa/sherpa-onnx-c-api.dll" "$sherpa/onnxruntime.dll" "$phonemize/piper_phonemize_core.dll" "$out/"
	;;
esac

cp LICENSE README.md "$out/"

cd dist
if [ "$os" = windows ]; then
	7z a -tzip "${name}.zip" "$name" >/dev/null
	echo "dist/${name}.zip"
else
	tar -czf "${name}.tar.gz" "$name"
	echo "dist/${name}.tar.gz"
fi

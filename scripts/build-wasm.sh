#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# Go 1.21+ moved wasm_exec.js from misc/wasm to lib/wasm. Probe both.
GOROOT="$(go env GOROOT)"
WASM_EXEC=""
for candidate in "$GOROOT/lib/wasm/wasm_exec.js" "$GOROOT/misc/wasm/wasm_exec.js"; do
    if [[ -f "$candidate" ]]; then
        WASM_EXEC="$candidate"
        break
    fi
done
if [[ -z "$WASM_EXEC" ]]; then
    echo "build-wasm: could not find wasm_exec.js in $GOROOT" >&2
    exit 1
fi

mkdir -p web/public
cp "$WASM_EXEC" web/public/wasm_exec.js

echo "build-wasm: building cmd/zanbato-wasm -> web/public/zanbato.wasm"
GOOS=js GOARCH=wasm go build \
    -trimpath -ldflags="-s -w" \
    -o web/public/zanbato.wasm \
    ./cmd/zanbato-wasm

size=$(wc -c < web/public/zanbato.wasm | tr -d ' ')
echo "build-wasm: zanbato.wasm = ${size} bytes"

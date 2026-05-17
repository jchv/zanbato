#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

TINYGOROOT="$(tinygo env TINYGOROOT)"
WASM_EXEC="$TINYGOROOT/targets/wasm_exec.js"
if [[ -z "$WASM_EXEC" ]]; then
    echo "build-wasm: could not find wasm_exec.js in $TINYGOROOT" >&2
    exit 1
fi

mkdir -p web/public
cp "$WASM_EXEC" web/public/wasm_exec.js

echo "build-wasm: building cmd/zanbato-wasm -> web/public/zanbato.wasm"
GOOS=js GOARCH=wasm tinygo build \
    --no-debug \
    -o web/public/zanbato.wasm \
    ./cmd/zanbato-wasm

size=$(wc -c < web/public/zanbato.wasm | tr -d ' ')
echo "build-wasm: zanbato.wasm = ${size} bytes"

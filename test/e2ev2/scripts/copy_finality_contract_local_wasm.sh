#!/bin/bash
set -o errexit -o nounset -o pipefail
command -v shellcheck >/dev/null && shellcheck "$0"

BASE=$(dirname "$0")
LOCAL_REPO="$BASE/../../../../rollup-bsn-contracts"
CONTRACTS="finality"
OUTPUT_FOLDER="$(dirname "$0")/../bytecode"
mkdir -p "$OUTPUT_FOLDER"

echo "DEV-only: copy from local built instead of downloading"

for CONTRACT in $CONTRACTS; do
  cp -f "${LOCAL_REPO}/artifacts/${CONTRACT}".wasm "$OUTPUT_FOLDER/"
done

cd "${LOCAL_REPO}"
TAG=$(git rev-parse HEAD)
cd - >/dev/null 2>&1
echo "$TAG" >"$OUTPUT_FOLDER/finality_contract_version.txt"

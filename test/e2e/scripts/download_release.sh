#!/bin/bash
set -o errexit -o nounset -o pipefail
command -v shellcheck >/dev/null && shellcheck "$0"

OWNER="babylonlabs-io"
REPO="babylon-contract"
CONTRACTS="babylon_contract btc_staking btc_finality"
OUTPUT_FOLDER="$(dirname "$0")/../bytecode"

[ $# -ne 1 ] && echo "Usage: $0 <version>" && exit 1
type wget >&2

TAG="$1"

for CONTRACT in $CONTRACTS
do
  echo -n "Downloading $CONTRACT..." >&2
  FILE="$CONTRACT.wasm.zip"
  URL="https://github.com/$OWNER/$REPO/releases/download/$TAG/$FILE"
  wget -nv -O "$OUTPUT_FOLDER/$FILE" "$URL"
  unzip -p "$OUTPUT_FOLDER/$FILE" >"$OUTPUT_FOLDER/$CONTRACT.wasm"
  rm -f "$OUTPUT_FOLDER/$FILE"
  echo "done." >&2
done
echo "$TAG" >"$OUTPUT_FOLDER/version.txt"

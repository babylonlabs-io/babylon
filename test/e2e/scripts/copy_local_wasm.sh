#!/bin/bash
set -o errexit -o nounset -o pipefail
command -v shellcheck >/dev/null && shellcheck "$0"

CONTRACTS="babylon_contract btc_staking"
OUTPUT_FOLDER="$(dirname "$0")/../bytecode"

echo "DEV-only: copy from local built instead of downloading"


M=$(uname -m)
S=${M/arm64/aarch64}
S=${S#x86_64}
S=${S:+-$S}

for CONTRACT in $CONTRACTS
do
  cp -f  ../../../babylon-contract/artifacts/"${CONTRACT}${S}".wasm "$OUTPUT_FOLDER/${CONTRACT}.wasm"
done

cd ../../../babylon-contract
TAG=$(git rev-parse HEAD)
cd - 2>/dev/null
echo "$TAG" >"$OUTPUT_FOLDER/version.txt"

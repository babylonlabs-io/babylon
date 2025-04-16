#!/usr/bin/env bash

set -eo pipefail

mkdir -p ./tmp-swagger-gen
cd proto
proto_dirs=$(find ./babylon -path -prune -o -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)
for dir in $proto_dirs; do
  # generate OpenAPI v2 files (filter query files)
  query_file=$(find "${dir}" -maxdepth 1 \( -name 'query.proto' -o -name 'service.proto' \))
  if [[ ! -z "$query_file" ]]; then
    echo "Generating Swagger for $query_file"
    buf generate --template buf.gen.swagger.yaml $query_file
  else
    echo "No query or service proto file found in $dir"
  fi
done

cd ..
# combine OpenAPI v2 files
swagger-combine ./client/docs/config.json -o ./client/docs/swagger-ui/swagger.yaml -f yaml --continueOnConflictingPaths true --includeDefinitions true

# clean OpenAPI v2 files
rm -rf ./tmp-swagger-gen
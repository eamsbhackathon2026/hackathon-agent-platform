#!/bin/sh
set -eu

service_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
spec="$service_dir/../../api/openapi.yaml"
config="$service_dir/oapi-codegen.yaml"

missing=""
for tag in $(awk '/^- name: / { print $3 }' "$spec"); do
  if ! awk -v wanted="$tag" '$1 == "-" && $2 == wanted { found=1 } END { exit !found }' "$config"; then
    missing="$missing $tag"
  fi
done

if [ -n "$missing" ]; then
  printf 'Các tag OpenAPI chưa được sinh vào server:%s\n' "$missing" >&2
  exit 1
fi
printf '%s\n' 'Mọi tag OpenAPI đã được sinh vào strict server.'

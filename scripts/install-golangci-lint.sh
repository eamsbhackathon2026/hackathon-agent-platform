#!/bin/sh
set -eu

version="${1:?Cần truyền phiên bản golangci-lint}"
case "$version" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) printf '%s\n' 'Phiên bản không hợp lệ.' >&2; exit 1 ;;
esac

repo_dir="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
system="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  arm64|aarch64) architecture=arm64 ;;
  x86_64) architecture=amd64 ;;
  *) printf '%s\n' 'Kiến trúc chưa được hỗ trợ bởi script cài linter.' >&2; exit 1 ;;
esac

release="golangci-lint-${version#v}-${system}-${architecture}"
archive="${release}.tar.gz"
base_url="https://github.com/golangci/golangci-lint/releases/download/${version}"
temp_dir="$(mktemp -d)"
trap 'rm -rf "$temp_dir"' EXIT HUP INT TERM
curl --fail --silent --show-error --location "$base_url/$archive" -o "$temp_dir/$archive"
curl --fail --silent --show-error --location "$base_url/golangci-lint-${version#v}-checksums.txt" -o "$temp_dir/checksums.txt"
awk -v archive="$archive" '$2 == archive {print}' "$temp_dir/checksums.txt" > "$temp_dir/selected-checksum.txt"
test -s "$temp_dir/selected-checksum.txt"
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$temp_dir" && sha256sum -c selected-checksum.txt)
else
  (cd "$temp_dir" && shasum -a 256 -c selected-checksum.txt)
fi
tar -xzf "$temp_dir/$archive" -C "$temp_dir" "$release/golangci-lint"
mkdir -p "$repo_dir/bin"
install -m 0755 "$temp_dir/$release/golangci-lint" "$repo_dir/bin/golangci-lint"
"$repo_dir/bin/golangci-lint" version

#!/bin/sh
set -eu

repository="epismoai/cli"
version="${EPISMO_VERSION:-latest}"
install_dir="${EPISMO_INSTALL_DIR:-$HOME/.local/bin}"

os=$(uname -s)
case "$os" in
  Darwin) target_os="darwin" ;;
  Linux) target_os="linux" ;;
  *) echo "epismo: unsupported operating system: $os" >&2; exit 1 ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) target_arch="amd64" ;;
  arm64|aarch64) target_arch="arm64" ;;
  *) echo "epismo: unsupported architecture: $arch" >&2; exit 1 ;;
esac

if [ "$version" = "latest" ]; then
  release_url="https://github.com/$repository/releases/latest/download"
else
  case "$version" in v*) ;; *) version="v$version" ;; esac
  release_url="https://github.com/$repository/releases/download/$version"
fi
release_url="${EPISMO_RELEASE_BASE_URL:-$release_url}"

archive="epismo_${target_os}_${target_arch}.tar.gz"
temp_base="${TMPDIR:-/tmp}"
case "$temp_base" in
  /*) ;;
  *) temp_base="/tmp" ;;
esac
case "$temp_base" in
  /) ;;
  *) temp_base="${temp_base%/}" ;;
esac
tmp_dir=$(mktemp -d "$temp_base/epismo.XXXXXX")
case "$tmp_dir" in
  "$temp_base"/epismo.*) ;;
  *) echo "epismo: refusing unsafe temporary directory: $tmp_dir" >&2; exit 1 ;;
esac
cleanup() { rm -rf -- "$tmp_dir"; }
trap cleanup EXIT INT TERM

curl --proto '=https' --tlsv1.2 -fsSL "$release_url/$archive" -o "$tmp_dir/$archive"
curl --proto '=https' --tlsv1.2 -fsSL "$release_url/checksums.txt" -o "$tmp_dir/checksums.txt"

expected=$(awk -v file="$archive" '$2 == file { print $1 }' "$tmp_dir/checksums.txt")
if [ -z "$expected" ]; then
  echo "epismo: checksum not found for $archive" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp_dir/$archive" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$tmp_dir/$archive" | awk '{print $1}')
fi
if [ "$actual" != "$expected" ]; then
  echo "epismo: checksum verification failed" >&2
  exit 1
fi

tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" epismo
mkdir -p "$install_dir"
install -m 0755 "$tmp_dir/epismo" "$install_dir/epismo"
installed_version=$("$install_dir/epismo" --version)
receipt_tmp=$(mktemp "$install_dir/.epismo-install.XXXXXX")
printf '%s\n' \
  '{' \
  '  "schemaVersion": 1,' \
  '  "method": "curl",' \
  "  \"installedVersion\": \"$installed_version\"" \
  '}' > "$receipt_tmp"
chmod 0644 "$receipt_tmp"
mv -f "$receipt_tmp" "$install_dir/epismo.install.json"
echo "Installed epismo to $install_dir/epismo"

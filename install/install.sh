#!/usr/bin/env bash
# Usage: curl -fsSL https://raw.githubusercontent.com/fergusstrange/embedded-postgres/master/install/install.sh | bash
# Pin with EP_VERSION=v2.0.0-alpha.1; choose location with EP_INSTALL_DIR.
set -euo pipefail
repo=fergusstrange/embedded-postgres
version=${EP_VERSION:-}
destination=${EP_INSTALL_DIR:-"$HOME/.local/bin"}
if [[ -z "$version" ]]; then
  version=$(curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
    "https://api.github.com/repos/$repo/releases?per_page=100" | \
    sed -nE 's/^[[:space:]]*"tag_name": "(v2\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?)",?$/\1/p' | head -n 1)
fi
if [[ ! "$version" =~ ^v2\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo 'No published v2 release found; set EP_VERSION to a published v2 tag.' >&2
  exit 1
fi
case "$(uname -s)" in
  Linux) os=linux; suffix= ;;
  Darwin) os=darwin; suffix= ;;
  MINGW*|MSYS*|CYGWIN*) os=windows; suffix=.exe ;;
  *) echo 'Unsupported OS; use Linux, macOS, or Windows Git Bash.' >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo 'Unsupported architecture; use AMD64 or ARM64.' >&2; exit 1 ;;
esac
asset="embedded-postgres_${os}_${arch}${suffix}"
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
base="https://github.com/$repo/releases/download/$version"
for name in "$asset" checksums.txt; do
  curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 "$base/$name" -o "$stage/$name"
done
expected=$(awk -v name="$asset" '$2 == name { print $1 }' "$stage/checksums.txt")
if [[ ! "$expected" =~ ^[0-9a-f]{64}$ ]]; then
  echo 'Missing or ambiguous SHA-256 checksum.' >&2; exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$stage/$asset"); actual=${actual%% *}
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$stage/$asset"); actual=${actual%% *}
else
  echo 'Install sha256sum or shasum to verify the download.' >&2; exit 1
fi
if [[ "$actual" != "$expected" ]]; then echo 'SHA-256 mismatch; installation refused.' >&2; exit 1; fi
mkdir -p "$destination"
# Stage beside the destination so the final rename stays on one filesystem.
staged=$(mktemp "$destination/.embedded-postgres.XXXXXX")
trap 'rm -rf "$stage"; rm -f "$staged"' EXIT
cp "$stage/$asset" "$staged"
chmod 755 "$staged"
mv -f "$staged" "$destination/embedded-postgres$suffix"
printf 'Installed %s to %s/embedded-postgres%s\n' "$version" "$destination" "$suffix"

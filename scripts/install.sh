#!/usr/bin/env sh
set -eu

repo="${CHANT_REPO:-fireharp/chant}"
version="${CHANT_VERSION:-latest}"
install_dir="${CHANT_INSTALL_DIR:-$HOME/.local/bin}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "chant install: missing required command: $1" >&2
    exit 1
  fi
}

need tar

if command -v curl >/dev/null 2>&1; then
  download() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
  download() { wget -qO "$2" "$1"; }
else
  echo "chant install: missing curl or wget" >&2
  exit 1
fi

case "$(uname -s)" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *)
    echo "chant install: unsupported OS $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *)
    echo "chant install: unsupported architecture $(uname -m)" >&2
    exit 1
    ;;
esac

asset="chant_${os}_${arch}.tar.gz"
if [ "$version" = "latest" ]; then
  base_url="https://github.com/${repo}/releases/latest/download"
else
  base_url="https://github.com/${repo}/releases/download/${version}"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

download "${base_url}/${asset}" "${tmp}/${asset}"
download "${base_url}/checksums.txt" "${tmp}/checksums.txt"

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$tmp" && grep "  ${asset}\$" checksums.txt | sha256sum -c -)
elif command -v shasum >/dev/null 2>&1; then
  (cd "$tmp" && grep "  ${asset}\$" checksums.txt | shasum -a 256 -c -)
else
  echo "chant install: warning: sha256sum/shasum not found; skipping checksum verification" >&2
fi

mkdir -p "$install_dir"
tar -xzf "${tmp}/${asset}" -C "$install_dir" chant

echo "chant installed to ${install_dir}/chant"

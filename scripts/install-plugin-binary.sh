#!/usr/bin/env bash
# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#
# Download the gemini-search-mcp binary from GitHub Releases into the plugin's
# bin/ directory so the bundled .mcp.json can launch it.
#
# The archives are produced by GoReleaser (see .goreleaser.yaml):
#   gemini-search-mcp_<os>_<arch>.tar.gz  (os in {darwin,linux}, arch in {amd64,arm64})
#   checksums.txt                          (sha256 of every archive)
#
# Usage:
#   scripts/install-plugin-binary.sh [VERSION]
#
#   VERSION  Release tag (e.g. v0.2.0) or "latest" (default).
#
# Honors CLAUDE_PLUGIN_ROOT when set (Claude Code exports it); otherwise the
# binary is installed into bin/ next to this script's parent directory.

set -euo pipefail

REPO="cwest/gemini-search-mcp"
BINARY="gemini-search-mcp"
VERSION="${1:-latest}"

err() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

# --- Resolve the install directory ----------------------------------------
if [ -n "${CLAUDE_PLUGIN_ROOT:-}" ]; then
  plugin_root="${CLAUDE_PLUGIN_ROOT}"
else
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  plugin_root="$(cd "${script_dir}/.." && pwd)"
fi
bin_dir="${plugin_root}/bin"

# --- Detect OS / arch and map to GoReleaser's archive naming --------------
uname_s="$(uname -s)"
case "${uname_s}" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *) err "unsupported OS: ${uname_s} (only darwin and linux have release binaries)" ;;
esac

uname_m="$(uname -m)"
case "${uname_m}" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) err "unsupported architecture: ${uname_m} (only amd64 and arm64 are released)" ;;
esac

# --- Required tooling ------------------------------------------------------
for cmd in curl tar; do
  command -v "${cmd}" >/dev/null 2>&1 || err "required command not found: ${cmd}"
done

if command -v sha256sum >/dev/null 2>&1; then
  sha_cmd="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  sha_cmd="shasum -a 256"
else
  err "no SHA-256 tool found (need sha256sum or shasum)"
fi

# --- Build download URLs ---------------------------------------------------
archive="${BINARY}_${os}_${arch}.tar.gz"
if [ "${VERSION}" = "latest" ]; then
  base="https://github.com/${REPO}/releases/latest/download"
else
  base="https://github.com/${REPO}/releases/download/${VERSION}"
fi

# --- Download, verify, extract --------------------------------------------
tmp_dir="$(mktemp -d)"
cleanup() { rm -rf "${tmp_dir}"; }
trap cleanup EXIT

printf 'Downloading %s (%s)...\n' "${archive}" "${VERSION}"
curl --fail --location --silent --show-error \
  --output "${tmp_dir}/${archive}" "${base}/${archive}" \
  || err "download failed: ${base}/${archive}"

curl --fail --location --silent --show-error \
  --output "${tmp_dir}/checksums.txt" "${base}/checksums.txt" \
  || err "download failed: ${base}/checksums.txt"

printf 'Verifying checksum...\n'
expected="$(grep " ${archive}\$" "${tmp_dir}/checksums.txt" | awk '{print $1}')"
[ -n "${expected}" ] || err "no checksum entry for ${archive} in checksums.txt"

actual="$(cd "${tmp_dir}" && ${sha_cmd} "${archive}" | awk '{print $1}')"
[ "${actual}" = "${expected}" ] \
  || err "checksum mismatch for ${archive}: expected ${expected}, got ${actual}"

printf 'Extracting %s...\n' "${BINARY}"
tar -xzf "${tmp_dir}/${archive}" -C "${tmp_dir}" "${BINARY}" \
  || err "archive did not contain ${BINARY}"

mkdir -p "${bin_dir}"
install -m 0755 "${tmp_dir}/${BINARY}" "${bin_dir}/${BINARY}"

printf 'Installed %s to %s\n' "${BINARY}" "${bin_dir}/${BINARY}"

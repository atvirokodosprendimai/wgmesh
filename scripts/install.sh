#!/usr/bin/env bash
# wgmesh one-command installer.
#
# Detects the host OS/architecture, downloads the matching release artifact
# from the latest GitHub release, verifies it against the published
# checksums.txt, installs the binary to a writable bin directory, and runs
# `wgmesh version` to confirm.
#
# Canonical location:
#   https://raw.githubusercontent.com/atvirokodosprendimai/wgmesh/main/scripts/install.sh
#
# Hosted as https://install.wgmesh.dev via a redirect to the raw file above,
# so this committed script is the single source of truth.
#
# Usage:
#   curl -fsSL https://install.wgmesh.dev | sh
#   curl -fsSL https://install.wgmesh.dev | sh -s -- --version v0.2.0
#
# Flags:
#   --version <tag>   Install a specific release tag (default: latest).
#   --bin <path>      Install the binary to this exact path (overrides auto-detect).
#   -h, --help        Show this help.
# shellcheck shell=bash
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
OWNER="atvirokodosprendimai"
REPO="wgmesh"
BINARY_NAME="wgmesh"

# Released archive name template is wgmesh_<version>_<os>_<arch>.tar.gz and
# contains a single `wgmesh` binary.

GITHUB_LATEST_BASE="https://github.com/${OWNER}/${REPO}/releases/latest/download"
GITHUB_TAG_BASE_TEMPLATE="https://github.com/${OWNER}/${REPO}/releases/download"

CHECKSUMS_FILENAME="checksums.txt"

# Destination install directories, tried in order when --bin is not supplied.
ROOT_BINDIR="/usr/local/bin"
USER_BINDIR="${HOME}/.local/bin"

# Optional override for the version to install (e.g. "v0.2.0").
TARGET_VERSION=""
# Optional exact install path.
TARGET_BIN=""

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info() {
	printf 'wgmesh: %s\n' "$*" >&2
}

err() {
	printf 'wgmesh: ERROR: %s\n' "$*" >&2
}

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		err "required command not found: %s" "$1"
		err "please install it and re-run this installer"
		exit 1
	fi
}

usage() {
	cat >&2 <<EOF
wgmesh installer

Usage:
  curl -fsSL https://install.wgmesh.dev | sh
  curl -fsSL https://install.wgmesh.dev | sh -s -- --version v0.2.0

Options:
  --version <tag>   Install a specific release tag (default: latest).
  --bin <path>      Install the binary to this exact path.
  -h, --help        Show this help.

Supported platforms: linux/amd64, linux/arm64, linux/armv7, darwin/amd64,
darwin/arm64. Windows and i386 are not supported; the installer exits cleanly.
EOF
}

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
while [ $# -gt 0 ]; do
	case "$1" in
		--version)
			if [ $# -lt 2 ]; then
				err "--version requires an argument"
				exit 1
			fi
			TARGET_VERSION="$2"
			shift 2
			;;
		--bin)
			if [ $# -lt 2 ]; then
				err "--bin requires an argument"
				exit 1
			fi
			TARGET_BIN="$2"
			shift 2
			;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			err "unknown argument: %s" "$1"
			usage
			exit 1
			;;
	esac
done

# ---------------------------------------------------------------------------
# Preflight checks
# ---------------------------------------------------------------------------
need_cmd uname
need_cmd curl
# `tar` and a checksum tool are required later; verify now for clearer errors.
need_cmd tar
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
	err "neither sha256sum nor shasum is available; cannot verify checksum"
	err "install coreutils (Linux) or run on macOS, then re-run"
	exit 1
fi

# ---------------------------------------------------------------------------
# Detect OS and architecture
# ---------------------------------------------------------------------------
os_raw="$(uname -s)"
arch_raw="$(uname -m)"

case "$os_raw" in
	Linux*) OS="linux" ;;
	Darwin*) OS="darwin" ;;
	*)
		err "unsupported operating system: %s" "$os_raw"
		err "wgmesh installer supports Linux and macOS only"
		err "Windows support is not yet available; see docs/trial-mesh.md"
		exit 1
		;;
esac

# Map machine architecture to the GoReleaser naming. i386/x86 are rejected:
# the project does not publish i386 artifacts and 64-bit is required.
case "$arch_raw" in
	x86_64 | amd64) ARCH="amd64" ;;
	aarch64 | arm64) ARCH="arm64" ;;
	armv7l) ARCH="armv7" ;;
	i386 | i486 | i586 | i686 | x86)
		err "unsupported architecture: %s (i386/32-bit is not supported)" "$arch_raw"
		err "wgmesh requires a 64-bit Linux or ARM system"
		exit 1
		;;
	*)
		err "unsupported architecture: %s" "$arch_raw"
		err "supported: amd64, arm64, armv7"
		exit 1
		;;
esac

info "detected platform: ${OS}/${ARCH}"

# arm + darwin is not published (ignored in .goreleaser.yml).
if [ "$OS" = "darwin" ] && [ "$ARCH" = "armv7" ]; then
	err "darwin/armv7 is not published; use a 64-bit macOS host"
	exit 1
fi

# ---------------------------------------------------------------------------
# Resolve download URLs
# ---------------------------------------------------------------------------
# For "latest", GitHub's releases/latest/download/<asset> redirects to the
# matching asset of the most recent release — but only when the asset name is
# stable. GoReleaser archives are version-stamped (wgmesh_v0.2.0_linux_amd64),
# so for a stable latest we rely on the checksums.txt list to discover the
# exact versioned archive name.
#
# Flow:
#   1. Download checksums.txt (latest or specific version).
#   2. Read the archive name for our OS/ARCH out of it.
#   3. Download that exact archive.
if [ -n "$TARGET_VERSION" ]; then
	# Normalize: GitHub release tags include the leading "v".
	case "$TARGET_VERSION" in
		v*) VERSION_TAG="$TARGET_VERSION" ;;
		*) VERSION_TAG="v${TARGET_VERSION}" ;;
	esac
	CHECKSUMS_URL="${GITHUB_TAG_BASE_TEMPLATE}/${VERSION_TAG}/${CHECKSUMS_FILENAME}"
else
	CHECKSUMS_URL="${GITHUB_LATEST_BASE}/${CHECKSUMS_FILENAME}"
fi

tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t wgmesh-install)"
# shellcheck disable=SC2317 # cleanup is invoked via the trap below
cleanup() {
	rm -rf "$tmpdir"
}
trap cleanup EXIT INT TERM

checksums_path="${tmpdir}/${CHECKSUMS_FILENAME}"
info "downloading checksums: %s" "$CHECKSUMS_URL"
if ! curl -fsSL "$CHECKSUMS_URL" -o "$checksums_path"; then
	err "failed to download checksums.txt"
	err "check your network and that a release exists at"
	err "  https://github.com/${OWNER}/${REPO}/releases"
	exit 1
fi

# Find the line for our platform's archive. The archive base name is
# wgmesh_<version>_<os>_<arch>.tar.gz; match the suffix _<os>_<arch>.tar.gz.
archive_suffix="_${OS}_${ARCH}.tar.gz"
archive_line="$(grep -F "$archive_suffix" "$checksums_path" | head -n 1 || true)"
if [ -z "$archive_line" ]; then
	err "no release archive found for platform ${OS}/${ARCH}"
	err "looked for a checksums.txt line matching: *%s" "$archive_suffix"
	err "the latest release may not have been built for this platform"
	exit 1
fi

# checksums.txt lines are "<sha256>  <filename>" (two spaces).
archive_name="$(printf '%s\n' "$archive_line" | awk '{print $2}')"
expected_sha="$(printf '%s\n' "$archive_line" | awk '{print $1}')"
if [ -z "$archive_name" ] || [ -z "$expected_sha" ]; then
	err "could not parse checksums.txt line: %s" "$archive_line"
	exit 1
fi

if [ -n "$TARGET_VERSION" ]; then
	archive_url="${GITHUB_TAG_BASE_TEMPLATE}/${VERSION_TAG}/${archive_name}"
else
	# For latest, we cannot use releases/latest/download/<versioned-name>
	# because we do not know the version ahead of time. Instead, GitHub
	# resolves the API redirect: /releases/latest/download/<asset> works for
	# any asset name in the latest release.
	archive_url="${GITHUB_LATEST_BASE}/${archive_name}"
fi

# ---------------------------------------------------------------------------
# Download and verify the archive
# ---------------------------------------------------------------------------
archive_path="${tmpdir}/${archive_name}"
info "downloading %s" "$archive_url"
if ! curl -fsSL "$archive_url" -o "$archive_path"; then
	err "failed to download archive: %s" "$archive_url"
	exit 1
fi

# Compute SHA256 of the downloaded file using whichever tool is available.
compute_sha256() {
	_file="$1"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$_file" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$_file" | awk '{print $1}'
	else
		# Preflight already checked this; unreachable in practice.
		return 1
	fi
}

actual_sha="$(compute_sha256 "$archive_path")"
if [ "$actual_sha" != "$expected_sha" ]; then
	err "checksum mismatch for %s" "$archive_name"
	err "expected: %s" "$expected_sha"
	err "actual:   %s" "$actual_sha"
	err "the download may be corrupted or tampered with; aborting"
	exit 1
fi
info "checksum verified (%s)" "$actual_sha"

# ---------------------------------------------------------------------------
# Extract the binary
# ---------------------------------------------------------------------------
extract_dir="${tmpdir}/extract"
mkdir -p "$extract_dir"
# tar auto-detects gzip; -o extracts into the current dir (avoid BSD/GNU quirks).
if ! tar -xf "$archive_path" -C "$extract_dir"; then
	err "failed to extract archive"
	exit 1
fi

extracted_bin="${extract_dir}/${BINARY_NAME}"
if [ ! -f "$extracted_bin" ]; then
	err "archive did not contain expected binary: %s" "$BINARY_NAME"
	err "archive contents:"
	( cd "$extract_dir" && ls -la ) >&2 || true
	exit 1
fi

# ---------------------------------------------------------------------------
# Choose install destination
# ---------------------------------------------------------------------------
can_write_dir() {
	_dir="$1"
	[ -d "$_dir" ] || return 1
	[ -w "$_dir" ] || return 1
}

install_binary() {
	_src="$1"
	_dest_dir="$2"
	_dest="${_dest_dir}/${BINARY_NAME}"
	info "installing to %s" "$_dest"
	# cp then chmod so we never leave a half-written executable in place.
	if ! cp "$_src" "$_dest" 2>/dev/null; then
		return 1
	fi
	if ! chmod 0755 "$_dest" 2>/dev/null; then
		rm -f "$_dest"
		return 1
	fi
	echo "$_dest"
}

installed_path=""

if [ -n "$TARGET_BIN" ]; then
	# Explicit --bin path: install exactly there. Create the parent dir.
	target_dir="$(dirname "$TARGET_BIN")"
	if ! mkdir -p "$target_dir" 2>/dev/null; then
		err "cannot create install directory: %s" "$target_dir"
		exit 1
	fi
	result="$(install_binary "$extracted_bin" "$target_dir" || true)"
	if [ -z "$result" ]; then
		err "failed to install to %s" "$TARGET_BIN"
		exit 1
	fi
	# install_binary names it after BINARY_NAME; rename if user asked otherwise.
	if [ "$result" != "$TARGET_BIN" ]; then
		if ! mv "$result" "$TARGET_BIN" 2>/dev/null; then
			err "failed to rename %s to %s" "$result" "$TARGET_BIN"
			exit 1
		fi
		result="$TARGET_BIN"
	fi
	installed_path="$result"
elif can_write_dir "$ROOT_BINDIR"; then
	installed_path="$(install_binary "$extracted_bin" "$ROOT_BINDIR")"
elif [ -n "${HOME:-}" ] && [ -d "$HOME" ]; then
	mkdir -p "$USER_BINDIR" 2>/dev/null || true
	installed_path="$(install_binary "$extracted_bin" "$USER_BINDIR" || true)"
	if [ -z "$installed_path" ]; then
		err "could not install to %s or %s" "$ROOT_BINDIR" "$USER_BINDIR"
		err "re-run with sudo, or use --bin <path>"
		exit 1
	fi
	info "not root and %s is not writable; installed to %s" "$ROOT_BINDIR" "$USER_BINDIR"
	info "add it to your PATH, e.g.:"
	info "  echo 'export PATH=\"%s:\$PATH\"' >> ~/.profile && export PATH=\"%s:\$PATH\"" "$USER_BINDIR" "$USER_BINDIR"
else
	err "no writable install directory found (tried %s and %s)" "$ROOT_BINDIR" "$USER_BINDIR"
	err "re-run with sudo, or set HOME, or use --bin <path>"
	exit 1
fi

# ---------------------------------------------------------------------------
# Verify the install
# ---------------------------------------------------------------------------
# Prefer the just-installed binary even if an older one shadows it on PATH.
verify_cmd="$installed_path version"
if ! "$installed_path" version >/dev/null 2>&1; then
	# Fall back to PATH lookup (e.g. if the file is not directly executable
	# in a restricted environment but is reachable via PATH).
	if ! command -v "$BINARY_NAME" >/dev/null 2>&1 || ! "$BINARY_NAME" version >/dev/null 2>&1; then
		err "installed binary at %s did not run" "$installed_path"
		err "verify it is executable: %s version" "$installed_path"
		exit 1
	fi
	verify_cmd="$BINARY_NAME version"
fi

installed_version="$($verify_cmd 2>/dev/null | head -n 1 || true)"
if [ -z "$installed_version" ]; then
	err "installed binary did not print a version"
	err "run manually: %s version" "$installed_path"
	exit 1
fi

printf '\n' >&2
info "install complete"
printf '%s\n' "$installed_version" >&2
printf '\n' >&2

# Tell the user the single next step: join the public trial mesh.
# Keep this string stable; the quickstart page references it.
printf 'Next: join the public trial mesh in one command:\n' >&2
printf '  %s quickstart\n' "$BINARY_NAME" >&2
printf '\n' >&2

exit 0

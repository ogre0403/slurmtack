# shellcheck shell=bash
# Shared primitives for the fake passthrough scripts. Sourced by
# reconfigure.sh and verify.sh; not meant to be executed directly.
#
# WARNING: TEST-ONLY — does not interact with GPU hardware, VFIO, or boot
# configuration. Use scripts/gpu-passthrough for production GPU nodes.

log() {
	printf '%s\n' "$*" >&2
}

fail() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

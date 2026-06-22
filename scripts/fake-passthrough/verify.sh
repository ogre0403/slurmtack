#!/usr/bin/env bash
#
# Fake GPU passthrough verification — TEST-ONLY.
#
#   verify.sh enable    Simulate verifying the enabled passthrough state.
#   verify.sh disable   Simulate verifying the disabled passthrough state.
#
# WARNING: This script does NOT check GPU hardware, VFIO module bindings, kernel
# arguments, or any real post-reboot state. It exists solely to let the switch
# orchestrator exercise the full reconfigure/verify workflow in non-GPU test
# environments. Do NOT use this bundle in production — point
# GPU_PASSTHROUGH_SCRIPT_DIR at scripts/gpu-passthrough instead.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/fake-passthrough/lib.sh
. "${SCRIPT_DIR}/lib.sh"

usage() {
	printf 'usage: %s <enable|disable>\n' "$(basename "$0")" >&2
}

main() {
	if [ "$#" -ne 1 ]; then
		usage
		fail "exactly one action argument is required"
	fi

	case "$1" in
	enable)
		log "[FAKE] fake passthrough enabled-state verification simulated — no GPU hardware checks performed (test-only)"
		;;
	disable)
		log "[FAKE] fake passthrough disabled-state verification simulated — no GPU hardware checks performed (test-only)"
		;;
	*)
		usage
		fail "invalid action: $1 (expected 'enable' or 'disable')"
		;;
	esac
}

main "$@"

#!/usr/bin/env bash
#
# Fake GPU passthrough reconfiguration — TEST-ONLY.
#
#   reconfigure.sh enable    Simulate enabling passthrough (deterministic no-op).
#   reconfigure.sh disable   Simulate disabling passthrough (deterministic no-op).
#
# WARNING: This script does NOT modify any host state, kernel arguments, VFIO
# configuration, or initramfs. It exists solely to let the switch orchestrator
# exercise the full reconfigure/verify workflow in non-GPU test environments.
# Do NOT use this bundle in production — point GPU_PASSTHROUGH_SCRIPT_DIR at
# scripts/gpu-passthrough instead.

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
		log "[FAKE] fake passthrough enable simulated — no GPU hardware changes made (test-only)"
		;;
	disable)
		log "[FAKE] fake passthrough disable simulated — no GPU hardware changes made (test-only)"
		;;
	*)
		usage
		fail "invalid action: $1 (expected 'enable' or 'disable')"
		;;
	esac
}

main "$@"

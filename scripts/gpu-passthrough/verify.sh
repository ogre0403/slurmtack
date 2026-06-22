#!/usr/bin/env bash
#
# GPU passthrough verification.
#
#   verify.sh enable    Verify the host is in the enabled passthrough state.
#   verify.sh disable   Verify the host is in the disabled passthrough state.
#
# Intended to run after a reboot. Exits zero only when the post-reboot state
# matches the requested mode, and non-zero (with a diagnostic) otherwise.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/gpu-passthrough/lib.sh
. "${SCRIPT_DIR}/lib.sh"

usage() {
	printf 'usage: %s <enable|disable>\n' "$(basename "$0")" >&2
}

verify_enable() {
	local ok=1

	# 1. IOMMU kernel arguments active.
	local arg
	for arg in "${GPU_PT_GRUB_ARGS[@]}"; do
		if cmdline_has_arg "$arg"; then
			log "ok: kernel arg active: $arg"
		else
			log "FAIL: kernel arg not active in /proc/cmdline: $arg"
			ok=0
		fi
	done

	# 2. VFIO modules loaded.
	local module
	for module in "${GPU_PT_MODULES[@]}"; do
		if module_loaded "$module"; then
			log "ok: VFIO module loaded: $module"
		else
			log "FAIL: VFIO module not loaded: $module"
			ok=0
		fi
	done

	# 3. VFIO boot configuration files present.
	local f
	for f in "$GPU_PT_VFIO_MODULES_FILE" "$GPU_PT_VFIO_MODPROBE_FILE"; do
		if [ -f "$f" ]; then
			log "ok: VFIO config present: $f"
		else
			log "FAIL: VFIO config missing: $f"
			ok=0
		fi
	done

	# 4. Every detected NVIDIA device bound to vfio-pci.
	local detected bound
	detected="$(count_nvidia_devices)"
	bound="$(nvidia_devices_bound_to_vfio)"
	if [ "$detected" -eq 0 ]; then
		log "FAIL: no NVIDIA devices detected"
		ok=0
	elif [ "$bound" -lt "$detected" ]; then
		log "FAIL: only ${bound}/${detected} NVIDIA devices bound to vfio-pci"
		ok=0
	else
		log "ok: ${bound}/${detected} NVIDIA devices bound to vfio-pci"
	fi

	if [ "$ok" -ne 1 ]; then
		fail "GPU passthrough enabled-state verification failed"
	fi
	log "GPU passthrough enabled-state verification passed"
}

verify_disable() {
	local ok=1

	# 1. IOMMU kernel arguments absent.
	local arg
	for arg in "${GPU_PT_GRUB_ARGS[@]}"; do
		if cmdline_has_arg "$arg"; then
			log "FAIL: passthrough kernel arg still active: $arg"
			ok=0
		else
			log "ok: kernel arg absent: $arg"
		fi
	done

	# 2. VFIO boot configuration files absent.
	local f
	for f in "$GPU_PT_VFIO_MODULES_FILE" "$GPU_PT_VFIO_MODPROBE_FILE"; do
		if [ -e "$f" ]; then
			log "FAIL: passthrough config still present: $f"
			ok=0
		else
			log "ok: VFIO config absent: $f"
		fi
	done

	# 3. No detected NVIDIA device bound to vfio-pci.
	local bound
	bound="$(nvidia_devices_bound_to_vfio)"
	if [ "$bound" -gt 0 ]; then
		log "FAIL: ${bound} NVIDIA device(s) still bound to vfio-pci"
		ok=0
	else
		log "ok: no NVIDIA devices bound to vfio-pci"
	fi

	if [ "$ok" -ne 1 ]; then
		fail "GPU passthrough disabled-state verification failed"
	fi
	log "GPU passthrough disabled-state verification passed"
}

main() {
	if [ "$#" -ne 1 ]; then
		usage
		fail "exactly one action argument is required"
	fi

	case "$1" in
	enable)
		verify_enable
		;;
	disable)
		verify_disable
		;;
	*)
		usage
		fail "invalid action: $1 (expected 'enable' or 'disable')"
		;;
	esac
}

main "$@"

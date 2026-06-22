#!/usr/bin/env bash
#
# GPU passthrough reconfiguration.
#
#   reconfigure.sh enable    Prepare the host for GPU passthrough (VFIO).
#   reconfigure.sh disable   Remove GPU passthrough configuration.
#
# This script mutates boot-time configuration (kernel arguments, VFIO module
# and modprobe files) and rebuilds initramfs when a change is made. It does NOT
# reboot; the caller is responsible for rebooting and then running verify.sh.
#
# Exit status is non-zero on an unsupported action, when no NVIDIA GPU is found
# for an enable action, or when any mutation step fails.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/gpu-passthrough/lib.sh
. "${SCRIPT_DIR}/lib.sh"

usage() {
	printf 'usage: %s <enable|disable>\n' "$(basename "$0")" >&2
}

# write_if_changed writes content to a file only when it differs from the
# current contents. Prints "changed" on stdout when a write occurred.
write_if_changed() {
	local dest="$1" content="$2"
	# Compare via command substitution on both sides so trailing-newline
	# differences do not produce a spurious rewrite.
	if [ -f "$dest" ] && [ "$(cat "$dest")" = "$(printf '%s' "$content")" ]; then
		return 0
	fi
	printf '%s' "$content" >"$dest"
	chmod 0644 "$dest"
	echo changed
}

reconfigure_enable() {
	require_root

	local ids
	ids="$(detect_nvidia_ids)"
	if [ -z "$ids" ]; then
		fail "no NVIDIA PCI devices were detected on the host"
	fi
	log "detected NVIDIA PCI IDs: $(echo "$ids" | tr '\n' ' ')"

	local changed=0

	# 1. Ensure IOMMU kernel arguments are configured. grubby --update-kernel
	# is idempotent, but only mark changed when an arg is missing from the
	# active command line so we know whether initramfs/reboot is required.
	local arg need_grub=0
	for arg in "${GPU_PT_GRUB_ARGS[@]}"; do
		if ! cmdline_has_arg "$arg"; then
			need_grub=1
			break
		fi
	done
	if [ "$need_grub" -eq 1 ]; then
		log "configuring IOMMU kernel arguments: ${GPU_PT_GRUB_ARGS[*]}"
		grubby --update-kernel=ALL --args="${GPU_PT_GRUB_ARGS[*]}"
		changed=1
	fi

	# 2. VFIO modules-load file.
	local modules_content=""
	local module
	for module in "${GPU_PT_MODULES[@]}"; do
		modules_content+="${module}"$'\n'
	done
	if [ -n "$(write_if_changed "$GPU_PT_VFIO_MODULES_FILE" "$modules_content")" ]; then
		log "wrote VFIO modules file: $GPU_PT_VFIO_MODULES_FILE"
		changed=1
	fi

	# 3. VFIO modprobe file (blacklists + vfio-pci device ids).
	local modprobe_content=""
	for module in "${GPU_PT_BLACKLIST_MODULES[@]}"; do
		modprobe_content+="blacklist ${module}"$'\n'
	done
	local joined_ids
	joined_ids="$(echo "$ids" | paste -sd, -)"
	modprobe_content+="options vfio-pci ids=${joined_ids}"$'\n'
	if [ -n "$(write_if_changed "$GPU_PT_VFIO_MODPROBE_FILE" "$modprobe_content")" ]; then
		log "wrote VFIO modprobe file: $GPU_PT_VFIO_MODPROBE_FILE"
		changed=1
	fi

	# 4. Rebuild initramfs when any passthrough-related change was made.
	if [ "$changed" -eq 1 ]; then
		log "rebuilding initramfs: ${GPU_PT_INITRAMFS_CMD[*]}"
		"${GPU_PT_INITRAMFS_CMD[@]}"
		log "GPU passthrough enable applied; reboot required"
	else
		log "GPU passthrough already enabled; no changes made"
	fi
}

reconfigure_disable() {
	require_root

	local changed=0

	# 1. Remove VFIO configuration files.
	local f
	for f in "$GPU_PT_VFIO_MODULES_FILE" "$GPU_PT_VFIO_MODPROBE_FILE"; do
		if [ -e "$f" ]; then
			log "removing VFIO config file: $f"
			rm -f "$f"
			changed=1
		fi
	done

	# 2. Remove IOMMU kernel arguments when present on the active command line.
	local arg need_grub=0
	for arg in "${GPU_PT_GRUB_ARGS[@]}"; do
		if cmdline_has_arg "$arg"; then
			need_grub=1
			break
		fi
	done
	if [ "$need_grub" -eq 1 ]; then
		log "removing IOMMU kernel arguments: ${GPU_PT_GRUB_ARGS[*]}"
		grubby --update-kernel=ALL --remove-args="${GPU_PT_GRUB_ARGS[*]}"
		changed=1
	fi

	# 3. Unload VFIO modules now when possible (best effort before reboot).
	for module in vfio_pci vfio_iommu_type1 vfio; do
		if module_loaded "$module"; then
			log "unloading VFIO module: $module"
			if modprobe -r "$module"; then
				changed=1
			else
				log "could not unload $module now; reboot will clear it"
			fi
		fi
	done

	# 4. Rebuild initramfs when any passthrough-related change was made.
	if [ "$changed" -eq 1 ]; then
		log "rebuilding initramfs: ${GPU_PT_INITRAMFS_CMD[*]}"
		"${GPU_PT_INITRAMFS_CMD[@]}"
		log "GPU passthrough disable applied; reboot required"
	else
		log "GPU passthrough already disabled; no changes made"
	fi
}

main() {
	if [ "$#" -ne 1 ]; then
		usage
		fail "exactly one action argument is required"
	fi

	case "$1" in
	enable)
		reconfigure_enable
		;;
	disable)
		reconfigure_disable
		;;
	*)
		usage
		fail "invalid action: $1 (expected 'enable' or 'disable')"
		;;
	esac
}

main "$@"

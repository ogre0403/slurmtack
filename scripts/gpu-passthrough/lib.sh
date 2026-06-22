# shellcheck shell=bash
# Shared primitives for the GPU passthrough reconfiguration and verification
# scripts. Sourced by reconfigure.sh and verify.sh; not meant to be executed
# directly. Mirrors the behavior of the former hack/gpu-passthrough Ansible role.

# Passthrough configuration contract (kept in sync with the old role defaults).
GPU_PT_GRUB_ARGS=(intel_iommu=on iommu=pt)
GPU_PT_MODULES=(vfio vfio_iommu_type1 vfio_pci)
GPU_PT_BLACKLIST_MODULES=(nouveau nvidia nvidia_drm nvidia_modeset nvidia_uvm nvidia_peermem)
GPU_PT_VENDOR_PATTERN="10de"
# File paths and the kernel command-line source default to the real host
# locations but may be overridden via the environment so the scripts can be
# exercised against a sandbox in test_scripts.sh.
GPU_PT_VFIO_MODULES_FILE="${GPU_PT_VFIO_MODULES_FILE:-/etc/modules-load.d/vfio.conf}"
GPU_PT_VFIO_MODPROBE_FILE="${GPU_PT_VFIO_MODPROBE_FILE:-/etc/modprobe.d/vfio.conf}"
GPU_PT_CMDLINE_FILE="${GPU_PT_CMDLINE_FILE:-/proc/cmdline}"
GPU_PT_INITRAMFS_CMD=(dracut -f)

log() {
	printf '%s\n' "$*" >&2
}

fail() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

# detect_nvidia_ids prints the de-duplicated, lower-cased NVIDIA PCI
# vendor:device IDs (e.g. "10de:1db4"), one per line.
detect_nvidia_ids() {
	# grep exits non-zero on no match; tolerate it so callers (which run under
	# `set -o pipefail`) can treat an empty result as "no GPU" rather than a
	# pipeline failure.
	lspci -nn |
		{ grep -i 'NVIDIA' || true; } |
		{ grep -Eo '\[[0-9a-fA-F]{4}:[0-9a-fA-F]{4}\]' || true; } |
		tr -d '[]' |
		tr '[:upper:]' '[:lower:]' |
		sort -u
}

# count_nvidia_devices prints the number of NVIDIA PCI functions present.
count_nvidia_devices() {
	lspci -nn | grep -ci 'NVIDIA' || true
}

# cmdline_has_arg reports whether the given kernel argument is present in
# /proc/cmdline.
cmdline_has_arg() {
	local arg="$1"
	grep -qw -- "$arg" "$GPU_PT_CMDLINE_FILE"
}

# module_loaded reports whether a kernel module is currently loaded.
module_loaded() {
	local module="$1"
	lsmod | awk '{print $1}' | grep -qx "$module"
}

# nvidia_devices_bound_to_vfio prints the count of NVIDIA functions reporting
# "Kernel driver in use: vfio-pci".
#
# lspci -nnk does not separate device entries with blank lines: each device
# starts on a non-indented line (the PCI address) and its details follow on
# tab-indented continuation lines. We therefore split on the device-header line
# rather than on blank lines, and count a device when its block both names the
# NVIDIA vendor and reports the vfio-pci driver.
nvidia_devices_bound_to_vfio() {
	lspci -nnk |
		awk -v pat="$GPU_PT_VENDOR_PATTERN" '
			function flush() {
				if (in_dev && is_nvidia && is_vfio) { count++ }
			}
			/^[^[:space:]]/ {
				flush()
				in_dev = 1
				is_nvidia = (tolower($0) ~ pat)
				is_vfio = 0
				next
			}
			/Kernel driver in use: vfio-pci/ { is_vfio = 1 }
			END { flush(); print count + 0 }
		'
}

# require_root exits non-zero unless running as root, since the reconfiguration
# steps mutate boot configuration and rebuild initramfs.
require_root() {
	if [ "$(id -u)" -ne 0 ]; then
		fail "must run as root"
	fi
}

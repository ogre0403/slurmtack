#!/usr/bin/env bash
#
# Direct script-level tests for the GPU passthrough scripts. These exercise
# reconfigure.sh and verify.sh independently of the orchestrator by stubbing the
# host tools they depend on (lspci, grubby, dracut, lsmod, modprobe, id) on PATH
# and pointing the VFIO config paths at a sandbox directory.
#
# Run: scripts/gpu-passthrough/test_scripts.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RECONFIGURE="${SCRIPT_DIR}/reconfigure.sh"
VERIFY="${SCRIPT_DIR}/verify.sh"

PASS=0
FAIL=0

# Per-test sandbox state, reset by setup_fixture.
FIXTURE=""
BIN=""

# make_stub writes an executable stub named $1 into $BIN with body $2.
make_stub() {
	local name="$1" body="$2"
	cat >"${BIN}/${name}" <<EOF
#!/usr/bin/env bash
${body}
EOF
	chmod +x "${BIN}/${name}"
}

# setup_fixture builds a fresh sandbox with default stubs describing a host that
# has one NVIDIA GPU and no passthrough configuration. Individual tests override
# the relevant fixture files before invoking the scripts.
setup_fixture() {
	FIXTURE="$(mktemp -d)"
	BIN="${FIXTURE}/bin"
	mkdir -p "${BIN}" "${FIXTURE}/etc/modules-load.d" "${FIXTURE}/etc/modprobe.d"

	# State files the stubs read so tests can simulate host conditions.
	echo "BOOT_IMAGE=/vmlinuz root=/dev/sda1 ro" >"${FIXTURE}/cmdline"
	# lspci -nn output line(s); one NVIDIA device by default.
	printf '3b:00.0 3D controller [0302]: NVIDIA Corporation GV100GL [Tesla V100] [10de:1db4]\n' >"${FIXTURE}/lspci_nn"
	# lspci -nnk output paragraphs; default driver is nvidia (not vfio-pci).
	cat >"${FIXTURE}/lspci_nnk" <<'EOF'
3b:00.0 3D controller [0302]: NVIDIA Corporation GV100GL [Tesla V100] [10de:1db4]
	Kernel driver in use: nvidia
EOF
	echo "" >"${FIXTURE}/lsmod"

	make_stub id 'echo 0' # always "root"
	make_stub lspci '
if [ "$1" = "-nnk" ]; then cat "$FIXTURE/lspci_nnk"; else cat "$FIXTURE/lspci_nn"; fi'
	make_stub lsmod 'cat "$FIXTURE/lsmod"'
	make_stub grubby '
for arg in "$@"; do
  case "$arg" in
    --args=*) printf "%s" "${arg#--args=}" >> "$FIXTURE/grubby_added" ;;
    --remove-args=*) printf "%s" "${arg#--remove-args=}" >> "$FIXTURE/grubby_removed" ;;
  esac
done
exit 0'
	make_stub dracut 'echo dracut >> "$FIXTURE/dracut_calls"; exit 0'
	make_stub modprobe 'echo "$@" >> "$FIXTURE/modprobe_calls"; exit 0'

	export FIXTURE
}

teardown_fixture() {
	[ -n "$FIXTURE" ] && rm -rf "$FIXTURE"
	FIXTURE=""
}

# run_script invokes a target script with the sandbox PATH and overridden config
# paths. Captures stdout+stderr in REPLY_OUT and exit status in REPLY_RC.
run_script() {
	local script="$1" action="$2"
	set +e
	REPLY_OUT="$(
		PATH="${BIN}:${PATH}" \
			GPU_PT_VFIO_MODULES_FILE="${FIXTURE}/etc/modules-load.d/vfio.conf" \
			GPU_PT_VFIO_MODPROBE_FILE="${FIXTURE}/etc/modprobe.d/vfio.conf" \
			GPU_PT_CMDLINE_FILE="${FIXTURE}/cmdline" \
			bash "$script" "$action" 2>&1
	)"
	REPLY_RC=$?
	set -e
}

check() {
	local desc="$1" cond="$2"
	if [ "$cond" = "1" ]; then
		PASS=$((PASS + 1))
		printf 'PASS: %s\n' "$desc"
	else
		FAIL=$((FAIL + 1))
		printf 'FAIL: %s\n' "$desc"
		printf '      output:\n%s\n' "$REPLY_OUT" | sed 's/^/      /'
	fi
}

eq() { [ "$1" = "$2" ] && echo 1 || echo 0; }
ne() { [ "$1" != "$2" ] && echo 1 || echo 0; }

# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

test_invalid_action() {
	setup_fixture
	run_script "$RECONFIGURE" bogus
	check "reconfigure rejects invalid action" "$(ne "$REPLY_RC" 0)"
	check "reconfigure invalid action reports message" \
		"$(echo "$REPLY_OUT" | grep -q 'invalid action' && echo 1 || echo 0)"
	teardown_fixture
}

test_missing_action() {
	setup_fixture
	set +e
	out="$(PATH="${BIN}:${PATH}" bash "$RECONFIGURE" 2>&1)"
	rc=$?
	set -e
	check "reconfigure requires exactly one action" "$([ "$rc" -ne 0 ] && echo 1 || echo 0)"
	teardown_fixture
}

test_enable_no_gpu() {
	setup_fixture
	: >"${FIXTURE}/lspci_nn" # no NVIDIA devices
	run_script "$RECONFIGURE" enable
	check "enable fails when no NVIDIA GPU detected" "$(ne "$REPLY_RC" 0)"
	check "enable no-GPU reports detection failure" \
		"$(echo "$REPLY_OUT" | grep -q 'no NVIDIA' && echo 1 || echo 0)"
	teardown_fixture
}

test_enable_applies_config() {
	setup_fixture
	run_script "$RECONFIGURE" enable
	check "enable succeeds on fresh host" "$(eq "$REPLY_RC" 0)"
	check "enable writes modules file" \
		"$([ -f "${FIXTURE}/etc/modules-load.d/vfio.conf" ] && echo 1 || echo 0)"
	check "enable writes modprobe file with detected id" \
		"$(grep -q 'options vfio-pci ids=10de:1db4' "${FIXTURE}/etc/modprobe.d/vfio.conf" && echo 1 || echo 0)"
	check "enable configures grub args" \
		"$([ -f "${FIXTURE}/grubby_added" ] && grep -q 'intel_iommu=on' "${FIXTURE}/grubby_added" && echo 1 || echo 0)"
	check "enable rebuilds initramfs" \
		"$([ -f "${FIXTURE}/dracut_calls" ] && echo 1 || echo 0)"
	teardown_fixture
}

test_enable_idempotent() {
	setup_fixture
	# Host already fully enabled: args active, files present.
	echo "BOOT_IMAGE=/vmlinuz root=/dev/sda1 ro intel_iommu=on iommu=pt" >"${FIXTURE}/cmdline"
	printf 'vfio\nvfio_iommu_type1\nvfio_pci\n' >"${FIXTURE}/etc/modules-load.d/vfio.conf"
	printf 'blacklist nouveau\nblacklist nvidia\nblacklist nvidia_drm\nblacklist nvidia_modeset\nblacklist nvidia_uvm\nblacklist nvidia_peermem\noptions vfio-pci ids=10de:1db4\n' >"${FIXTURE}/etc/modprobe.d/vfio.conf"
	run_script "$RECONFIGURE" enable
	check "enable idempotent run succeeds" "$(eq "$REPLY_RC" 0)"
	check "enable idempotent run makes no initramfs rebuild" \
		"$([ ! -f "${FIXTURE}/dracut_calls" ] && echo 1 || echo 0)"
	teardown_fixture
}

test_disable_removes_config() {
	setup_fixture
	echo "BOOT_IMAGE=/vmlinuz root=/dev/sda1 ro intel_iommu=on iommu=pt" >"${FIXTURE}/cmdline"
	printf 'vfio\n' >"${FIXTURE}/etc/modules-load.d/vfio.conf"
	printf 'options vfio-pci ids=10de:1db4\n' >"${FIXTURE}/etc/modprobe.d/vfio.conf"
	printf 'vfio_pci 1 0\nvfio_iommu_type1 1 0\nvfio 2 0\n' >"${FIXTURE}/lsmod"
	run_script "$RECONFIGURE" disable
	check "disable succeeds" "$(eq "$REPLY_RC" 0)"
	check "disable removes modules file" \
		"$([ ! -f "${FIXTURE}/etc/modules-load.d/vfio.conf" ] && echo 1 || echo 0)"
	check "disable removes modprobe file" \
		"$([ ! -f "${FIXTURE}/etc/modprobe.d/vfio.conf" ] && echo 1 || echo 0)"
	check "disable removes grub args" \
		"$([ -f "${FIXTURE}/grubby_removed" ] && grep -q 'intel_iommu=on' "${FIXTURE}/grubby_removed" && echo 1 || echo 0)"
	check "disable unloads vfio modules" \
		"$([ -f "${FIXTURE}/modprobe_calls" ] && grep -q 'vfio' "${FIXTURE}/modprobe_calls" && echo 1 || echo 0)"
	check "disable rebuilds initramfs after change" \
		"$([ -f "${FIXTURE}/dracut_calls" ] && echo 1 || echo 0)"
	teardown_fixture
}

test_disable_idempotent() {
	setup_fixture
	# Already disabled: no args, no files, no modules.
	run_script "$RECONFIGURE" disable
	check "disable idempotent run succeeds" "$(eq "$REPLY_RC" 0)"
	check "disable idempotent run makes no initramfs rebuild" \
		"$([ ! -f "${FIXTURE}/dracut_calls" ] && echo 1 || echo 0)"
	teardown_fixture
}

test_verify_enable_success() {
	setup_fixture
	echo "BOOT_IMAGE=/vmlinuz intel_iommu=on iommu=pt" >"${FIXTURE}/cmdline"
	printf 'vfio 2 0\nvfio_iommu_type1 1 0\nvfio_pci 1 0\n' >"${FIXTURE}/lsmod"
	printf 'vfio\n' >"${FIXTURE}/etc/modules-load.d/vfio.conf"
	printf 'options vfio-pci ids=10de:1db4\n' >"${FIXTURE}/etc/modprobe.d/vfio.conf"
	cat >"${FIXTURE}/lspci_nnk" <<'EOF'
3b:00.0 3D controller [0302]: NVIDIA Corporation GV100GL [Tesla V100] [10de:1db4]
	Kernel driver in use: vfio-pci
EOF
	run_script "$VERIFY" enable
	check "verify enable passes when fully enabled" "$(eq "$REPLY_RC" 0)"
	teardown_fixture
}

# Reproduces the real multi-GPU H200/H100 layout: two NVIDIA functions, both
# bound to vfio-pci, with tab-indented continuation lines and NO blank line
# between devices. Guards against the paragraph-mode regression where only one
# device was ever counted.
test_verify_enable_multi_gpu_success() {
	setup_fixture
	echo "BOOT_IMAGE=/vmlinuz intel_iommu=on iommu=pt" >"${FIXTURE}/cmdline"
	printf 'vfio 2 0\nvfio_iommu_type1 1 0\nvfio_pci 1 0\n' >"${FIXTURE}/lsmod"
	printf 'vfio\n' >"${FIXTURE}/etc/modules-load.d/vfio.conf"
	printf 'options vfio-pci ids=10de:2335,10de:22a3\n' >"${FIXTURE}/etc/modprobe.d/vfio.conf"
	printf '9a:00.0 3D controller [0302]: NVIDIA Corporation GH100 [10de:2335]\n03:00.0 Bridge [0680]: NVIDIA Corporation GH100 [10de:22a3]\n' >"${FIXTURE}/lspci_nn"
	cat >"${FIXTURE}/lspci_nnk" <<'EOF'
0000:9a:00.0 3D controller [0302]: NVIDIA Corporation GH100 [H200 SXM 141GB] [10de:2335] (rev a1)
	Subsystem: NVIDIA Corporation Device [10de:18bf]
	Kernel driver in use: vfio-pci
	Kernel modules: nouveau, nvidia_drm, nvidia
0000:03:00.0 Bridge [0680]: NVIDIA Corporation GH100 [H100 NVSwitch] [10de:22a3] (rev a1)
	Subsystem: NVIDIA Corporation Device [10de:1796]
	Kernel driver in use: vfio-pci
	Kernel modules: nvidia_drm, nvidia
EOF
	run_script "$VERIFY" enable
	check "verify enable passes when all multi-GPU devices bound" "$(eq "$REPLY_RC" 0)"
	check "verify enable reports 2/2 bound" \
		"$(echo "$REPLY_OUT" | grep -q '2/2 NVIDIA devices bound to vfio-pci' && echo 1 || echo 0)"
	teardown_fixture
}

# Multi-GPU host where only one of two devices is bound to vfio-pci must fail.
test_verify_enable_multi_gpu_partial() {
	setup_fixture
	echo "BOOT_IMAGE=/vmlinuz intel_iommu=on iommu=pt" >"${FIXTURE}/cmdline"
	printf 'vfio 2 0\nvfio_iommu_type1 1 0\nvfio_pci 1 0\n' >"${FIXTURE}/lsmod"
	printf 'vfio\n' >"${FIXTURE}/etc/modules-load.d/vfio.conf"
	printf 'options vfio-pci ids=10de:2335,10de:22a3\n' >"${FIXTURE}/etc/modprobe.d/vfio.conf"
	printf '9a:00.0 3D controller [0302]: NVIDIA Corporation GH100 [10de:2335]\n03:00.0 Bridge [0680]: NVIDIA Corporation GH100 [10de:22a3]\n' >"${FIXTURE}/lspci_nn"
	cat >"${FIXTURE}/lspci_nnk" <<'EOF'
0000:9a:00.0 3D controller [0302]: NVIDIA Corporation GH100 [H200 SXM 141GB] [10de:2335] (rev a1)
	Kernel driver in use: vfio-pci
	Kernel modules: nvidia_drm, nvidia
0000:03:00.0 Bridge [0680]: NVIDIA Corporation GH100 [H100 NVSwitch] [10de:22a3] (rev a1)
	Kernel driver in use: nvidia
	Kernel modules: nvidia_drm, nvidia
EOF
	run_script "$VERIFY" enable
	check "verify enable fails when only one of two devices bound" "$(ne "$REPLY_RC" 0)"
	check "verify enable partial reports 1/2 bound" \
		"$(echo "$REPLY_OUT" | grep -q 'only 1/2 NVIDIA devices bound to vfio-pci' && echo 1 || echo 0)"
	teardown_fixture
}

test_verify_enable_driver_mismatch() {
	setup_fixture
	echo "BOOT_IMAGE=/vmlinuz intel_iommu=on iommu=pt" >"${FIXTURE}/cmdline"
	printf 'vfio 2 0\nvfio_iommu_type1 1 0\nvfio_pci 1 0\n' >"${FIXTURE}/lsmod"
	printf 'vfio\n' >"${FIXTURE}/etc/modules-load.d/vfio.conf"
	printf 'options vfio-pci ids=10de:1db4\n' >"${FIXTURE}/etc/modprobe.d/vfio.conf"
	# GPU still bound to nvidia, not vfio-pci (default lspci_nnk).
	run_script "$VERIFY" enable
	check "verify enable fails on driver mismatch" "$(ne "$REPLY_RC" 0)"
	check "verify enable mismatch reports binding" \
		"$(echo "$REPLY_OUT" | grep -q 'vfio-pci' && echo 1 || echo 0)"
	teardown_fixture
}

test_verify_disable_success() {
	setup_fixture
	# No passthrough args, no files, GPU bound to nvidia (default).
	run_script "$VERIFY" disable
	check "verify disable passes when fully disabled" "$(eq "$REPLY_RC" 0)"
	teardown_fixture
}

test_verify_disable_leftover_binding() {
	setup_fixture
	# GPU still bound to vfio-pci -> disable verification must fail.
	cat >"${FIXTURE}/lspci_nnk" <<'EOF'
3b:00.0 3D controller [0302]: NVIDIA Corporation GV100GL [Tesla V100] [10de:1db4]
	Kernel driver in use: vfio-pci
EOF
	run_script "$VERIFY" disable
	check "verify disable fails with leftover vfio binding" "$(ne "$REPLY_RC" 0)"
	check "verify disable leftover reports still bound" \
		"$(echo "$REPLY_OUT" | grep -q 'still bound to vfio-pci' && echo 1 || echo 0)"
	teardown_fixture
}

test_verify_disable_leftover_config() {
	setup_fixture
	# Config file remains -> disable verification must fail.
	printf 'options vfio-pci ids=10de:1db4\n' >"${FIXTURE}/etc/modprobe.d/vfio.conf"
	run_script "$VERIFY" disable
	check "verify disable fails with leftover config" "$(ne "$REPLY_RC" 0)"
	teardown_fixture
}

main() {
	test_invalid_action
	test_missing_action
	test_enable_no_gpu
	test_enable_applies_config
	test_enable_idempotent
	test_disable_removes_config
	test_disable_idempotent
	test_verify_enable_success
	test_verify_enable_multi_gpu_success
	test_verify_enable_multi_gpu_partial
	test_verify_enable_driver_mismatch
	test_verify_disable_success
	test_verify_disable_leftover_binding
	test_verify_disable_leftover_config

	printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
	[ "$FAIL" -eq 0 ]
}

main "$@"

#!/usr/bin/env bash
#
# Script-level tests for the fake GPU passthrough bundle. These confirm the
# fake bundle preserves the gpu-passthrough CLI surface and succeeds on hosts
# without GPUs or any passthrough-related kernel state.
#
# Run: scripts/fake-passthrough/test_scripts.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RECONFIGURE="${SCRIPT_DIR}/reconfigure.sh"
VERIFY="${SCRIPT_DIR}/verify.sh"

PASS=0
FAIL=0

REPLY_OUT=""
REPLY_RC=0

run_script() {
	local script="$1" action="$2"
	set +e
	REPLY_OUT="$(bash "$script" "$action" 2>&1)"
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
# reconfigure.sh tests
# ---------------------------------------------------------------------------

test_reconfigure_enable_succeeds() {
	run_script "$RECONFIGURE" enable
	check "reconfigure enable exits zero" "$(eq "$REPLY_RC" 0)"
	check "reconfigure enable reports fake/test-only" \
		"$(echo "$REPLY_OUT" | grep -qi 'fake\|test' && echo 1 || echo 0)"
}

test_reconfigure_disable_succeeds() {
	run_script "$RECONFIGURE" disable
	check "reconfigure disable exits zero" "$(eq "$REPLY_RC" 0)"
	check "reconfigure disable reports fake/test-only" \
		"$(echo "$REPLY_OUT" | grep -qi 'fake\|test' && echo 1 || echo 0)"
}

test_reconfigure_invalid_action() {
	run_script "$RECONFIGURE" bogus
	check "reconfigure rejects invalid action" "$(ne "$REPLY_RC" 0)"
	check "reconfigure invalid action reports error" \
		"$(echo "$REPLY_OUT" | grep -q 'invalid action' && echo 1 || echo 0)"
}

test_reconfigure_missing_action() {
	set +e
	REPLY_OUT="$(bash "$RECONFIGURE" 2>&1)"
	REPLY_RC=$?
	set -e
	check "reconfigure requires exactly one action" "$(ne "$REPLY_RC" 0)"
}

# ---------------------------------------------------------------------------
# verify.sh tests
# ---------------------------------------------------------------------------

test_verify_enable_succeeds() {
	run_script "$VERIFY" enable
	check "verify enable exits zero" "$(eq "$REPLY_RC" 0)"
	check "verify enable reports fake/test-only" \
		"$(echo "$REPLY_OUT" | grep -qi 'fake\|test' && echo 1 || echo 0)"
}

test_verify_disable_succeeds() {
	run_script "$VERIFY" disable
	check "verify disable exits zero" "$(eq "$REPLY_RC" 0)"
	check "verify disable reports fake/test-only" \
		"$(echo "$REPLY_OUT" | grep -qi 'fake\|test' && echo 1 || echo 0)"
}

test_verify_invalid_action() {
	run_script "$VERIFY" bogus
	check "verify rejects invalid action" "$(ne "$REPLY_RC" 0)"
	check "verify invalid action reports error" \
		"$(echo "$REPLY_OUT" | grep -q 'invalid action' && echo 1 || echo 0)"
}

test_verify_missing_action() {
	set +e
	REPLY_OUT="$(bash "$VERIFY" 2>&1)"
	REPLY_RC=$?
	set -e
	check "verify requires exactly one action" "$(ne "$REPLY_RC" 0)"
}

# ---------------------------------------------------------------------------
# Interface compatibility tests
# ---------------------------------------------------------------------------

test_same_filenames_as_real_bundle() {
	local real_bundle
	real_bundle="$(cd "${SCRIPT_DIR}/../gpu-passthrough" && pwd)"
	for name in reconfigure.sh verify.sh lib.sh; do
		check "fake bundle has $name matching real bundle filename" \
			"$([ -f "${SCRIPT_DIR}/${name}" ] && [ -f "${real_bundle}/${name}" ] && echo 1 || echo 0)"
	done
}

main() {
	test_reconfigure_enable_succeeds
	test_reconfigure_disable_succeeds
	test_reconfigure_invalid_action
	test_reconfigure_missing_action
	test_verify_enable_succeeds
	test_verify_disable_succeeds
	test_verify_invalid_action
	test_verify_missing_action
	test_same_filenames_as_real_bundle

	printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
	[ "$FAIL" -eq 0 ]
}

main "$@"

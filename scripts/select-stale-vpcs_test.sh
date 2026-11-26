#!/usr/bin/env bash
# Copyright 2026 NVIDIA CORPORATION
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

# Tests for select-stale-vpcs.sh. A PATH-shim `aws` stub serves canned
# describe-vpcs and describe-instances text; no AWS call is ever made.
#
# Needs GNU date (as on the Linux CI runners). On macOS, put a `date` shim
# that execs gdate first on PATH.

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
SUBJECT="$SCRIPT_DIR/select-stale-vpcs.sh"

if [[ "$(date -u -d @0 +%s 2>/dev/null)" != "0" ]]; then
	echo "FATAL: GNU date (date -d) is required; on macOS shim date to gdate" >&2
	exit 1
fi

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# 2026-11-26T12:00:00Z
NOW=1795694400

mkdir -p "$WORK/bin"
cat >"$WORK/bin/aws" <<'STUB'
#!/usr/bin/env bash
# Fake aws. Serves $FIXTURES/vpcs and $FIXTURES/instances-<vpc>; a sibling
# <file>.rc makes that call fail with the rc it holds. Any call whose shape
# differs from the production call is rejected with rc 97.
set -euo pipefail
args="$*"
case "$args" in
"ec2 describe-vpcs --region us-west-2 --filters Name=tag:Project,Values=holodeck Name=tag:Environment,Values=cicd --query Vpcs[].VpcId --output text")
	f="$FIXTURES/vpcs"
	;;
"ec2 describe-instances --region us-west-2 --filters Name=vpc-id,Values="*" Name=instance-state-name,Values=pending,running,stopping,stopped,shutting-down --query Reservations[].Instances[].[InstanceId,LaunchTime] --output text")
	vpc=${args#*Name=vpc-id,Values=}
	vpc=${vpc%% *}
	f="$FIXTURES/instances-$vpc"
	;;
*)
	echo "aws stub: unexpected call: $args" >&2
	exit 97
	;;
esac
if [[ -f "$f.rc" ]]; then
	echo "aws stub: simulated failure" >&2
	exit "$(cat "$f.rc")"
fi
if [[ ! -f "$f" ]]; then
	echo "aws stub: no fixture $f" >&2
	exit 98
fi
cat "$f"
STUB
chmod +x "$WORK/bin/aws"

if [[ "$(PATH="$WORK/bin:$PATH" command -v aws)" != "$WORK/bin/aws" ]]; then
	echo "FATAL: aws stub is not first on PATH" >&2
	exit 1
fi

pass=0
fail=0
rc=0
FIX=""

new_fixtures() {
	FIX=$(mktemp -d "$WORK/fix.XXXXXX")
}

# vpcs <id>...: describe-vpcs text output (one tab-separated line, or empty).
vpcs() {
	if [[ $# -eq 0 ]]; then
		: >"$FIX/vpcs"
	else
		(
			IFS=$'\t'
			printf '%s\n' "$*"
		) >"$FIX/vpcs"
	fi
}

# instances <vpc> [<instance-id> <launch-time>]...: describe-instances text output.
instances() {
	local vpc=$1
	shift
	: >"$FIX/instances-$vpc"
	while [[ $# -gt 0 ]]; do
		printf '%s\t%s\n' "$1" "$2" >>"$FIX/instances-$vpc"
		shift 2
	done
}

# fail_call <fixture> <rc>: that aws call exits <rc>.
fail_call() {
	echo "$2" >"$FIX/$1.rc"
}

# run <command>...: runs with the stub first on PATH and no usable AWS credentials.
run() {
	rc=0
	env -u AWS_ACCESS_KEY_ID -u AWS_SECRET_ACCESS_KEY -u AWS_SESSION_TOKEN -u AWS_PROFILE \
		AWS_CONFIG_FILE=/dev/null AWS_SHARED_CREDENTIALS_FILE=/dev/null \
		FIXTURES="$FIX" PATH="$WORK/bin:$PATH" \
		"$@" >"$WORK/out" 2>"$WORK/err" || rc=$?
}

# run_default: the production invocation, min age 6h, region us-west-2, fixed now.
run_default() {
	run env AWS_REGION=us-west-2 HOLODECK_NOW_EPOCH="$NOW" bash "$SUBJECT" 6
}

# check <name> <want-rc> <want-stdout> [<want-stderr-line>]
#   want-stdout "" means stdout must be zero bytes; otherwise stdout must be
#   exactly that text plus one newline.
#   want-stderr-line "" means stderr must be empty; otherwise that exact line
#   must appear on stderr.
check() {
	local name=$1 want_rc=$2 want_out=$3 want_err=${4:-} ok=1
	if [[ $rc -ne $want_rc ]]; then
		ok=0
	fi
	if [[ -z $want_out ]]; then
		if [[ -s "$WORK/out" ]]; then ok=0; fi
	else
		if ! cmp -s "$WORK/out" <(printf '%s\n' "$want_out"); then ok=0; fi
	fi
	if [[ -z $want_err ]]; then
		if [[ -s "$WORK/err" ]]; then ok=0; fi
	else
		if ! grep -qxF -- "$want_err" "$WORK/err"; then ok=0; fi
	fi
	if [[ $ok -eq 1 ]]; then
		echo "PASS $name (rc=$rc)"
		pass=$((pass + 1))
	else
		echo "FAIL $name: rc=$rc want $want_rc; stdout='$(cat "$WORK/out")' want '$want_out'"
		sed 's/^/    stderr: /' "$WORK/err"
		fail=$((fail + 1))
	fi
}

T_1H=2026-11-26T11:00:00+00:00
T_6H=2026-11-26T06:00:00+00:00
T_7H=2026-11-26T05:00:00+00:00

# A VPC whose only instance launched 1h ago is in use: not selected.
new_fixtures
vpcs vpc-0a1
instances vpc-0a1 i-0a1 "$T_1H"
run_default
check young_1h_skipped 0 "" "skip vpc-0a1: instance i-0a1 launched $T_1H, younger than 6h"

# A VPC whose only instance launched 7h ago is selected.
new_fixtures
vpcs vpc-0b2
instances vpc-0b2 i-0b2 "$T_7H"
run_default
check old_7h_selected 0 "vpc-0b2" ""

# A VPC with no live instance is a leak: selected.
new_fixtures
vpcs vpc-0c3
instances vpc-0c3
run_default
check no_instances_selected 0 "vpc-0c3" ""

# Mixed: only the empty and the 7h VPCs, in input order (not sorted).
new_fixtures
vpcs vpc-0c3 vpc-0a1 vpc-0b2
instances vpc-0c3
instances vpc-0a1 i-0a1 "$T_1H"
instances vpc-0b2 i-0b2 "$T_7H"
run_default
check mixed_input_order 0 "vpc-0c3 vpc-0b2" "skip vpc-0a1: instance i-0a1 launched $T_1H, younger than 6h"

# describe-vpcs fails: exit 3, nothing on stdout.
new_fixtures
fail_call vpcs 254
run_default
check describe_vpcs_fails 3 "" "select-stale-vpcs: aws ec2 describe-vpcs failed in us-west-2"

# describe-instances fails for the second VPC: exit 3, and the first
# (selectable) VPC is not printed either.
new_fixtures
vpcs vpc-0b2 vpc-0d4
instances vpc-0b2 i-0b2 "$T_7H"
instances vpc-0d4
fail_call instances-vpc-0d4 255
run_default
check describe_instances_fails 3 "" "select-stale-vpcs: aws ec2 describe-instances failed for vpc-0d4 in us-west-2"

# An unparseable LaunchTime (aws text output prints None for null): exit 4,
# nothing on stdout, even though an earlier VPC was selectable.
new_fixtures
vpcs vpc-0b2 vpc-0e5
instances vpc-0b2 i-0b2 "$T_7H"
instances vpc-0e5 i-0e5 None
run_default
check unparseable_launch_time 4 "" "select-stale-vpcs: cannot parse LaunchTime 'None' of instance i-0e5 in vpc-0e5"

# No VPCs at all: exit 0, nothing on stdout.
new_fixtures
vpcs
run_default
check no_vpcs 0 "" ""

# Any young instance skips the VPC, not only the first one listed.
new_fixtures
vpcs vpc-0f6
instances vpc-0f6 i-0f6a "$T_7H" i-0f6b "$T_1H"
run_default
check second_instance_young_skipped 0 "" "skip vpc-0f6: instance i-0f6b launched $T_1H, younger than 6h"

# Exactly the floor is not younger than the floor: selected.
new_fixtures
vpcs vpc-0a7
instances vpc-0a7 i-0a7 "$T_6H"
run_default
check exactly_6h_selected 0 "vpc-0a7" ""

# The aws CLI v1 style with milliseconds and Z parses.
new_fixtures
vpcs vpc-0a8
instances vpc-0a8 i-0a8 2026-11-26T05:00:00.000Z
run_default
check millis_z_selected 0 "vpc-0a8" ""

# A timestamp of the right shape that date rejects: exit 4.
new_fixtures
vpcs vpc-0a9
instances vpc-0a9 i-0a9 2026-13-45T25:61:00+00:00
run_default
check invalid_calendar_time 4 "" "select-stale-vpcs: cannot parse LaunchTime '2026-13-45T25:61:00+00:00' of instance i-0a9 in vpc-0a9"

# An empty LaunchTime: GNU date reads "" as today 00:00, which would look
# 12h old. It must be rejected, not treated as old.
new_fixtures
vpcs vpc-0b1
instances vpc-0b1 i-0b1 ""
run_default
check empty_launch_time 4 "" "select-stale-vpcs: cannot parse LaunchTime '' of instance i-0b1 in vpc-0b1"

# Usage errors: exit 2, nothing on stdout, no aws call needed.
new_fixtures
vpcs vpc-0b2
instances vpc-0b2 i-0b2 "$T_7H"
run env AWS_REGION=us-west-2 HOLODECK_NOW_EPOCH="$NOW" bash "$SUBJECT" six
check usage_bad_min_age 2 "" "select-stale-vpcs: usage: select-stale-vpcs.sh <min-age-hours>"

run env -u AWS_REGION HOLODECK_NOW_EPOCH="$NOW" bash "$SUBJECT" 6
check usage_no_region 2 "" "select-stale-vpcs: AWS_REGION is not set"

run env AWS_REGION=us-west-2 HOLODECK_NOW_EPOCH=yesterday bash "$SUBJECT" 6
check usage_bad_now 2 "" "select-stale-vpcs: HOLODECK_NOW_EPOCH is not an epoch: 'yesterday'"

echo "passed=$pass failed=$fail"
if [[ $fail -ne 0 ]]; then
	exit 1
fi

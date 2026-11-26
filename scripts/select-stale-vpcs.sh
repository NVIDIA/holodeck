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

# Usage: select-stale-vpcs.sh <min-age-hours>
#
# Prints, space-separated on stdout, the IDs of the VPCs in $AWS_REGION
# tagged Project=holodeck, Environment=cicd that hold no non-terminated
# instance launched less than <min-age-hours> ago. A VPC with no live
# instance is a leak and is selected. Skipped VPCs are logged on stderr.
#
# Fails closed: on any error nothing is printed on stdout.
#
# Exit codes:
#   0  success (stdout is empty when nothing is selected)
#   2  usage error: bad <min-age-hours>, AWS_REGION unset, bad HOLODECK_NOW_EPOCH
#   3  an aws call failed
#   4  a LaunchTime could not be parsed
#
# HOLODECK_NOW_EPOCH overrides the current time (seconds since the epoch).
# Timestamps are parsed with GNU date -d, as on the Linux CI runners.

set -euo pipefail

LIVE_STATES="pending,running,stopping,stopped,shutting-down"
# ISO 8601 as printed by the aws CLI: 2026-11-26T05:00:00+00:00 or 2026-11-26T05:00:00.000Z
TS_RE='^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$'

die() {
	local rc=$1
	shift
	echo "select-stale-vpcs: $*" >&2
	exit "$rc"
}

# to_epoch <timestamp>: prints seconds since the epoch, or returns 1.
to_epoch() {
	local ts=$1 epoch
	[[ $ts =~ $TS_RE ]] || return 1
	epoch=$(date -u -d "$ts" +%s 2>/dev/null) || return 1
	[[ $epoch =~ ^[0-9]+$ ]] || return 1
	printf '%s\n' "$epoch"
}

if [[ $# -ne 1 || ! $1 =~ ^[1-9][0-9]*$ ]]; then
	die 2 "usage: select-stale-vpcs.sh <min-age-hours>"
fi
min_age_hours=$1
min_age_secs=$((min_age_hours * 3600))

if [[ -z ${AWS_REGION:-} ]]; then
	die 2 "AWS_REGION is not set"
fi

now=${HOLODECK_NOW_EPOCH:-$(date -u +%s)}
if [[ ! $now =~ ^[1-9][0-9]*$ ]]; then
	die 2 "HOLODECK_NOW_EPOCH is not an epoch: '$now'"
fi

if ! vpc_ids=$(aws ec2 describe-vpcs --region "$AWS_REGION" \
	--filters "Name=tag:Project,Values=holodeck" "Name=tag:Environment,Values=cicd" \
	--query "Vpcs[].VpcId" --output text); then
	die 3 "aws ec2 describe-vpcs failed in $AWS_REGION"
fi

selected=""
for vpc in $vpc_ids; do
	if ! instances=$(aws ec2 describe-instances --region "$AWS_REGION" \
		--filters "Name=vpc-id,Values=$vpc" "Name=instance-state-name,Values=$LIVE_STATES" \
		--query "Reservations[].Instances[].[InstanceId,LaunchTime]" --output text); then
		die 3 "aws ec2 describe-instances failed for $vpc in $AWS_REGION"
	fi

	young=""
	while IFS=$'\t' read -r id ts; do
		[[ -n $id ]] || continue
		if ! launched=$(to_epoch "$ts"); then
			die 4 "cannot parse LaunchTime '$ts' of instance $id in $vpc"
		fi
		if ((now - launched < min_age_secs)); then
			young="instance $id launched $ts"
			break
		fi
	done <<<"$instances"

	if [[ -n $young ]]; then
		echo "skip $vpc: $young, younger than ${min_age_hours}h" >&2
	else
		selected="${selected:+$selected }$vpc"
	fi
done

if [[ -n $selected ]]; then
	printf '%s\n' "$selected"
fi

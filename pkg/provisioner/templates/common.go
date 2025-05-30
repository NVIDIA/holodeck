/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package templates

import (
	"bytes"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

const CommonFunctions = `

export DEBIAN_FRONTEND=noninteractive
export HOLODECK_ENVIRONMENT=true

echo "APT::Get::AllowUnauthenticated 1;" | sudo tee /etc/apt/apt.conf.d/99allow-unauthenticated

install_packages_with_retry() {
	local max_retries=5 retry_delay=5
	local packages=("$@")
	
	for ((i=1; i<=max_retries; i++)); do
		echo "[$i/$max_retries] apt-get update"
		if sudo apt-get -o Acquire::Retries=3 update; then
			echo "[$i/$max_retries] installing: ${packages[*]}"
			if sudo DEBIAN_FRONTEND=noninteractive \
				   apt-get install -y --no-install-recommends "${packages[@]}"; then
				return 0            # success
			fi
		fi
		echo "Attempt $i failed; sleeping ${retry_delay}s" >&2
		sleep "$retry_delay"
	done
	echo "All ${max_retries} attempts failed" >&2
	return 1
}

with_retry() {
	local max_attempts="$1"
	local delay="$2"
	local count=0
	local rc
	shift 2

	while true; do
		set +e
		"$@"
		rc="$?"
		set -e

		count="$((count+1))"

		if [[ "${rc}" -eq 0 ]]; then
			return 0
		fi

		if [[ "${max_attempts}" -le 0 ]] || [[ "${count}" -lt "${max_attempts}" ]]; then
			sleep "${delay}"
		else
			break
		fi
	done

	return 1
}
`

// Template is the interface that wraps the Execute method.
type Template interface {
	Execute(tpl *bytes.Buffer, env v1alpha1.Environment) error
}

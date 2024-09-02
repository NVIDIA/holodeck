#!/usr/bin/env bash

# Copyright 2022 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

SOURCE_DIR="$(cd "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
ROOT_DIR="$SOURCE_DIR/.."

GINKGO="$ROOT_DIR"/bin/ginkgo
GINKGO_ARGS=${GINKGO_ARGS:-}

CI=${CI:-"true"}

LOG_ARTIFACT_DIR=${LOG_ARTIFACT_DIR:-${ROOT_DIR}/e2e_logs}
ENV_FILE=${ENV_FILE:-}
GINKGO_FOCUS=${GINKGO_FOCUS:-}

if [ "$1" == "aws" ]; then
  ENV_FILE=${ROOT_DIR}/tests/test_aws.yml
  GINKGO_FOCUS=${GINKGO_FOCUS:-"AWS"}
elif [ "$1" == "vsphere" ]; then
  ENV_FILE=${ROOT_DIR}/tests/test_vsphere.yml
  GINKGO_FOCUS=${GINKGO_FOCUS:-"VSPHERE"}
fi

# Set all ENV variables for e2e tests
export LOG_ARTIFACT_DIR ENV_FILE GINKGO_FOCUS CI

# shellcheck disable=SC2086
$GINKGO $GINKGO_ARGS -v  --focus $GINKGO_FOCUS ./tests/...

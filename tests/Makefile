# SPDX-FileCopyrightText: Copyright (c) 2025 NVIDIA CORPORATION & AFFILIATES. All rights reserved. 
# SPDX-License-Identifier: Apache-2.0
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

.PHONY: test $(GINKGO_BIN)

GINKGO_ARGS ?=
LOG_ARTIFACT_DIR ?= $(CURDIR)/e2e_logs
CI ?= true
GINKGO_FOCUS ?=

GINKGO_BIN := $(CURDIR)/bin/ginkgo

test: $(GINKGO_BIN)
	LOG_ARTIFACT_DIR=$(LOG_ARTIFACT_DIR) \
	CI=$(CI) \
	ENV_FILE=$(ENV_FILE) \
	GINKGO_FOCUS=$(GINKGO_FOCUS) \
	$(GINKGO_BIN) $(GINKGO_ARGS) -v --json-report ginkgo.json ./tests/...

$(GINKGO_BIN):
	mkdir -p $(CURDIR)/bin
	GOBIN=$(CURDIR)/bin go install github.com/onsi/ginkgo/v2/ginkgo@latest

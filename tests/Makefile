# Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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

GO_CMD ?= go

AWS_ENV_FILE ?= $(CURDIR)/tests/test_aws.yml
VSPHERE_ENV_FILE ?= $(CURDIR)/tests/test_vsphere.yml
LOG_ARTIFACTS ?= $(CURDIR)/e2e_logs

.PHONY: e2e-aws e2e-vsphere
e2e-aws:
	@echo "Running AWS e2e tests"
	$(GO_CMD) test -v $(CURDIR)/tests -args \
		-env-file=$(AWS_ENV_FILE)\
		-log-artifacts=$(LOG_ARTIFACTS) \
		-ginkgo.focus="AWS" \
		-test.timeout=1h \
		-ginkgo.v

e2e-vsphere:
	@echo "Running VSPHERE e2e tests"
	$(GO_CMD) test -v $(CURDIR)/tests -args \
		-env-file=$(VSPHERE_ENV_FILE)\
		-log-artifacts=$(LOG_ARTIFACTS) \
		-ginkgo.focus="VSPHERE" \
		-test.timeout=1h \
		-ginkgo.v

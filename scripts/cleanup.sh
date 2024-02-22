## Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.## 
## Licensed under the Apache License, Version 2.0 (the "License");
## you may not use this file except in compliance with the License.
## You may obtain a copy of the License at
##
##     http://www.apache.org/licenses/LICENSE-2.0 
##
## Unless required by applicable law or agreed to in writing, software
## distributed under the License is distributed on an "AS IS" BASIS,
## WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
## See the License for the specific language governing permissions and
## limitations under the License.
## 

#! /usr/bin/env bash
set +x

export DEBIAN_FRONTEND=noninteractive

if [ ! -d /github/workspace/.cache ]; then
    echo "Cache directory not found in /workspace"
    exit 1
fi

/user/bin/holodeck delete -f /github/workspace/$INPUT_HOLODECK_CONFIG -c /github/workspace/.cache

rm -rf /github/workspace/.cache
rm -f /github/workspace/key.pem
rm -f /github/workspace/kubeconfig

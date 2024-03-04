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

if [ -n "$INPUT_HOLODECK-CONFIG" ]; then
    if [ ! -f "/github/workspace/$INPUT_HOLODECK-CONFIG" ]; then
        echo "Holodeck config file not found in /workspace"
        exit 1
    fi
fi

if [ -z "$INPUT_AWS-ACCESS-KEY-ID" ] || [ -z "$INPUT_AWS-SECRET-ACCESS-KEY" ]; then
    echo "AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are not set"
    exit 1
fi

export AWS_ACCESS_KEY_ID=$INPUT_AWS-ACCESS-KEY-ID
export AWS_SECRET_ACCESS_KEY=$INPUT_AWS-SECRET-ACCESS-KEY

if [ -n "$SSH_KEY" ]; then
    $(umask 077;   echo "$SSH_KEY" > /github/workspace/key.pem)
fi

mkdir -p /github/workspace/.cache

/user/local/bin/holodeck create --provision  \
    -f /github/workspace/$INPUT_HOLODECK-CONFIG \
    -c /github/workspace/.cache \
    -k /github/workspace/kubeconfig

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

FROM golang:1.21

WORKDIR /src
COPY . .

RUN make build
RUN install -m 755 /src/bin/holodeck /usr/local/bin/holodeck && \
    install -m 755 /src/scripts/run.sh /usr/local/bin/run.sh && \
    install -m 755 /src/scripts/cleanup.sh /usr/local/bin/cleanup.sh

RUN echo "nobody:x:65534:65534:Nobody:/:" >> /etc/passwd
# Run as unprivileged user
USER 65534:65534

ENTRYPOINT ["/usr/local/bin/run.sh"]
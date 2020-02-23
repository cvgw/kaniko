# Copyright 2020 Google, Inc. All rights reserved.
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

FROM fedora:31
ARG IMAGE_NAME='nodejs'
LABEL name=${IMAGE_NAME}
ARG NODEJS_VERSION=v12.14.1
ARG CI_COMMIT_SHA=unspecified
LABEL git_commit=${CI_COMMIT_SHA}
ARG CI_PROJECT_URL
LABEL git_repository_url=${CI_PROJECT_URL}

ENV APP_HOME=/opt/app-root/src

WORKDIR /tmp
RUN yum install -y findutils readline-devel gcc gcc-c++ make zlib-devel xz openssl openssl-devel git patch \
&& curl -L https://nodejs.org/dist/${NODEJS_VERSION}/node-${NODEJS_VERSION}-linux-x64.tar.xz > node-${NODEJS_VERSION}-linux-x64.tar.xz \
&& tar -Jxvf node-${NODEJS_VERSION}-linux-x64.tar.xz \
&& cd node-${NODEJS_VERSION}-linux-x64 \
&& mv bin/* /usr/local/bin \
&& mv include/* /usr/local/include/ \
&& mv lib/* /usr/local/lib \
&& cd /tmp \
&& rm -rf node-${NODEJS_VERSION}-linux-x64 node-${NODEJS_VERSION}-linux-x64.tar.xz

#WORKDIR ${APP_HOME}
#ENV PATH /${APP_HOME}/node_modules/.bin:$PATH
#COPY package.json /${APP_HOME}/package.json
#RUN npm install \
#&& npm install react-scripts

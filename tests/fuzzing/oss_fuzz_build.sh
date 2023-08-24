#!/usr/bin/env bash

# This file is only meant to be run by OSS-fuzz and will not work
# if run outside of it.
# The api, compile_go_fuzzer() is provided by the OSS-fuzz
# environment and is a high level helper function for a series
# of compilation and linking steps to build the fuzzers in the
# OSS-fuzz environment.
# More info about compile_go_fuzzer() can be found here:
#     https://google.github.io/oss-fuzz/getting-started/new-project-guide/go-lang/#buildsh
set -o nounset
set -o pipefail
set -o errexit
set -x

# specifiy go version
apt-get update && apt-get install -y wget
cd $SRC

wget --quiet https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
mkdir tmp-go
rm -rf /root/.go/*
tar -C tmp-go/ -xzf go1.21.0.linux-amd64.tar.gz
mv tmp-go/go/* /root/.go/

# supporting cncf-fuzzing
cd $SRC/runc
go mod tidy

compile_go_fuzzer github.com/opencontainers/runc/libcontainer/userns FuzzUIDMap id_map_fuzzer linux,gofuzz
compile_go_fuzzer github.com/opencontainers/runc/libcontainer/user FuzzUser user_fuzzer
compile_go_fuzzer github.com/opencontainers/runc/libcontainer/configs FuzzUnmarshalJSON configs_fuzzer

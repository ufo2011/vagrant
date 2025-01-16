#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


csource="${BASH_SOURCE[0]}"
while [ -h "$csource" ] ; do csource="$(readlink "$csource")"; done
root="$( cd -P "$( dirname "$csource" )/../../" && pwd )"

. "${root}/.ci/load-ci.sh"
. "${root}/.ci/spec/env.sh"

pushd "${root}" > "${output}"

# Assumes packet is already set up
unset PACKET_EXEC_PRE_BUILTINS

# spec test configuration, defined by action runners, used by Vagrant on packet
export PKT_VAGRANT_HOST_BOXES="${VAGRANT_HOST_BOXES}"
export PKT_VAGRANT_GUEST_BOXES="${VAGRANT_GUEST_BOXES}"
# other vagrant-spec options
export PKT_VAGRANT_HOST_MEMORY="${VAGRANT_HOST_MEMORY:-10000}"
export PKT_VAGRANT_CWD="test/vagrant-spec/"
export PKT_VAGRANT_VAGRANTFILE=Vagrantfile.spec
export PKT_VAGRANT_SPEC_PROVIDERS="${VAGRANT_SPEC_PROVIDERS}"
export PKT_VAGRANT_DOCKER_IMAGES="${VAGRANT_DOCKER_IMAGES}"
###
# Run the job

echo "Running vagrant spec tests..."
# Need to make memory customizable for windows hosts
wrap_stream packet-exec run "vagrant provision" \
                "Vagrant Acceptance testing command failed"

echo "Finished vagrant spec tests"

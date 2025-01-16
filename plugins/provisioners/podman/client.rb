# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

require_relative "../container/client"

module VagrantPlugins
  module PodmanProvisioner
    class Client < VagrantPlugins::ContainerProvisioner::Client
      def initialize(machine)
        super(machine, "podman")
        @container_command = "podman"
      end
    end
  end
end

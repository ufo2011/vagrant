# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

require "vagrant"

module VagrantPlugins
  module HostNull
    class Plugin < Vagrant.plugin("2")
      name "null host"
      description "A host that implements no capabilities."

      host("null") do
        require_relative "host"
        Host
      end
    end
  end
end

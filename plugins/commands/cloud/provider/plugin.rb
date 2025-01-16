# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

require "vagrant"

module VagrantPlugins
  module CloudCommand
    module ProviderCommand
      class Plugin < Vagrant.plugin("2")
        name "vagrant cloud box"
        description <<-DESC
        Provider life cycle commands for Vagrant Cloud
        DESC

        command(:provider) do
          require_relative "root"
          Command::Root
        end
      end
    end
  end
end

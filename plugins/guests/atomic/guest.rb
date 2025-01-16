# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

require "vagrant"

module VagrantPlugins
  module GuestAtomic
    class Guest < Vagrant.plugin("2", :guest)
      def detect?(machine)
        machine.communicate.test("grep 'ostree=.*atomic' /proc/cmdline")
      end
    end
  end
end

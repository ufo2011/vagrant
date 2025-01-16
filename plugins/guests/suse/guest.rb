# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

module VagrantPlugins
  module GuestSUSE
    class Guest < Vagrant.plugin("2", :guest)
      def detect?(machine)
        machine.communicate.test("test -f /etc/SuSE-release || grep -q SUSE /etc/os-release")
      end
    end
  end
end

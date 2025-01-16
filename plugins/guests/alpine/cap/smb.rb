# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

module VagrantPlugins
  module GuestAlpine
    module Cap
      class SMB
        def self.smb_install(machine)
          machine.communicate.tap do |comm|
            comm.sudo('apk add cifs-utils')
          end
        end
      end
    end
  end
end
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

require "vagrant"
require_relative '../linux/guest'

module VagrantPlugins
  module GuestFedora
    class Guest < VagrantPlugins::GuestLinux::Guest
      # Name used for guest detection
      GUEST_DETECTION_NAME = "fedora".freeze
    end
  end
end

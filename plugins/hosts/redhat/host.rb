# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

Vagrant.require "pathname"

module VagrantPlugins
  module HostRedHat
    class Host < Vagrant.plugin("2", :host)
      def detect?(env)
        release_file = Pathname.new("/etc/redhat-release")

        if release_file.exist?
          release_file.open("r:ISO-8859-1:UTF-8") do |f|
            contents = f.gets
            return true if contents =~ /^CentOS/ # CentOS
            return true if contents =~ /^Fedora/ # Fedora
            return true if contents =~ /^Korora/ # Korora

            # Oracle Linux < 5.3
            return true if contents =~ /^Enterprise Linux Enterprise Linux/

            # Red Hat Enterprise Linux and Oracle Linux >= 5.3
            return true if contents =~ /^Red Hat Enterprise Linux/
          end
        end

        false
      end
    end
  end
end

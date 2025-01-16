# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

module VagrantPlugins
  module HyperV
    module Action
      class ResumeVM
        def initialize(app, env)
          @app = app
        end

        def call(env)
          env[:ui].info("Resuming the machine...")
          env[:machine].provider.driver.resume
          @app.call(env)
        end
      end
    end
  end
end

SHELL := /usr/bin/env bash

.PHONY: quality preflight build-agent bootstrap verify trigger destroy e2e-ephemeral ui ui-password purge-retained

quality:
	./scripts/quality.sh

preflight:
	./scripts/preflight.sh

build-agent:
	./scripts/build-agent.sh

bootstrap:
	./scripts/bootstrap.sh

verify:
	./scripts/verify.sh

trigger:
	./scripts/trigger-and-wait.sh

destroy:
	./scripts/destroy.sh

e2e-ephemeral:
	./scripts/run-ephemeral.sh

ui:
	./scripts/run-ephemeral.sh ui

ui-password:
	./scripts/ui-password.sh

purge-retained:
	./scripts/retained-state.sh purge

SHELL := /usr/bin/env bash

.PHONY: preflight bootstrap verify diagnose destroy grafana prometheus

preflight:
	./scripts/preflight.sh

bootstrap:
	./scripts/bootstrap.sh

verify:
	./scripts/verify.sh

diagnose:
	./scripts/diagnose.sh

destroy:
	./scripts/destroy.sh

grafana:
	kubectl --context kind-investment-platform --namespace platform-observability \
		port-forward service/monitoring-grafana 3000:80

prometheus:
	kubectl --context kind-investment-platform --namespace platform-observability \
		port-forward service/monitoring-kube-prometheus-prometheus 9090:9090

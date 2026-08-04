SHELL := /usr/bin/env bash
PORTFOLIO_DASHBOARD_PORT ?= 8080

.PHONY: preflight bootstrap verify diagnose destroy e2e-ephemeral e2e-capacity-ephemeral reclaim-runtime grafana prometheus \
	build-data-path bootstrap-data-path verify-data-path diagnose-data-path destroy-data-path \
	verify-scale-lab verify-capacity-benchmark verify-capacity-smoke \
	build-analytics bootstrap-analytics verify-analytics diagnose-analytics destroy-analytics portfolio-dashboard \
	quality

quality:
	../../scripts/ci/portfolio-quality-gates.sh

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

e2e-ephemeral:
	./scripts/run-ephemeral.sh

e2e-capacity-ephemeral:
	EPHEMERAL_WORKFLOW=capacity ./scripts/run-ephemeral.sh

reclaim-runtime:
	./scripts/cleanup-runtime.sh

grafana:
	kubectl --context kind-investment-platform --namespace platform-observability \
		port-forward service/monitoring-grafana 3000:80

prometheus:
	kubectl --context kind-investment-platform --namespace platform-observability \
		port-forward service/monitoring-kube-prometheus-prometheus 9090:9090

build-data-path:
	./scripts/build-market-pipeline.sh

bootstrap-data-path:
	./scripts/bootstrap-data-path.sh

verify-data-path:
	./scripts/verify-data-path.sh

verify-scale-lab:
	./scripts/verify-scale-lab.sh

verify-capacity-benchmark:
	./scripts/verify-capacity-benchmark.sh

verify-capacity-smoke:
	CAPACITY_EVENT_COUNTS=10000 CAPACITY_REPETITIONS=1 CAPACITY_TARGET_RATE=250 ./scripts/verify-capacity-benchmark.sh

diagnose-data-path:
	./scripts/diagnose-data-path.sh

destroy-data-path:
	./scripts/destroy-data-path.sh

build-analytics:
	./scripts/build-analytics.sh

bootstrap-analytics:
	./scripts/bootstrap-analytics.sh

verify-analytics:
	./scripts/verify-analytics.sh

diagnose-analytics:
	./scripts/diagnose-analytics.sh

destroy-analytics:
	./scripts/destroy-analytics.sh

portfolio-dashboard:
	kubectl --context kind-investment-platform --namespace analytics-apps \
		port-forward service/portfolio-api $(PORTFOLIO_DASHBOARD_PORT):8080

.PHONY: install run test test-go test-python test-integration test-load test-e2e lint format build verify local-up local-down kind-up

install:
	python -m pip install -r requirements.txt
	python -m pip install -e ./python

run:
	cd go && go run ./cmd/gateway

test: test-go test-python

test-go:
	cd go && go test -buildvcs=false ./...

test-python:
	python -m pytest python/tests -q

test-integration:
	bash scripts/integration-test.sh

test-load:
	k6 run tests/load/gateway.js

test-e2e:
	bash scripts/e2e-kind.sh

lint:
	cd go && go vet ./...
	ruff check python

format:
	cd go && gofmt -w .
	ruff format python

build:
	mkdir -p bin
	cd go && for cmd in gateway operator integration-worker trace-proxy feature-gateway storage-proxy metrics-collector cli; do \
		go build -buildvcs=false -o ../bin/mlaiops-$$cmd ./cmd/$$cmd; \
	done

verify: test lint build
	node --check go/cmd/gateway/web/app.js
	python -m compileall -q python/mlaiops_sdk
	! rg -i '\b(mlrun|nuclio|v3io|iguazio)\b' go config

local-up:
	docker compose -f deploy/compose.yaml up -d --build
	bash scripts/local-topics.sh

local-down:
	docker compose -f deploy/compose.yaml down

kind-up:
	bash scripts/kind-up.sh

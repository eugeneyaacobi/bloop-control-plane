GO ?= go

.PHONY: fmt test build run dev-runtime-e2e dev-runtime-e2e-down

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

build:
	$(GO) build ./...

run:
	$(GO) run ./cmd/bloop-control-plane

dev-runtime-e2e:
	./scripts/dev-runtime-e2e.sh

dev-runtime-e2e-down:
	./scripts/dev-runtime-e2e.sh --down

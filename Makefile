GO ?= go

.PHONY: fmt test build run

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

build:
	$(GO) build ./...

run:
	$(GO) run ./cmd/bloop-control-plane

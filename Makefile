GO := /usr/local/go/bin/go
BIN := cburn

.PHONY: build install lint test test-race bench fuzz clean

## Build & install
build:
	$(GO) build -o $(BIN) .

install:
	$(GO) install .

## Quality
lint:
	golangci-lint run ./...

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

bench:
	$(GO) test -bench=. -benchmem ./internal/pipeline/

## Fuzz (run for 30s by default, override with FUZZ_TIME=2m)
FUZZ_TIME ?= 30s
fuzz:
	$(GO) test -fuzz=Fuzz -fuzztime=$(FUZZ_TIME) ./internal/source/

## Housekeeping
clean:
	rm -f $(BIN)
	$(GO) clean -testcache

SHELL := bash
GO ?= go
GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.8.0

.PHONY: all test lint fmt-check vet langcheck signoff commitcheck vuln check

all: check

## test: run all tests with the race detector
test:
	$(GO) test -race ./...

## lint: formatting, vet, English-only check
lint: fmt-check vet langcheck

fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	$(GO) vet ./...

langcheck:
	set -o pipefail; git ls-files -z | $(GO) run ./tools/langcheck files

## commitcheck: check that all commit messages are English
commitcheck:
	set -o pipefail; git log -z --format='%H%n%B' | $(GO) run ./tools/langcheck commits

## vuln: scan dependencies for known vulnerabilities
vuln:
	$(GO) run $(GOVULNCHECK) ./...

## check: everything CI runs
check: lint test vuln commitcheck

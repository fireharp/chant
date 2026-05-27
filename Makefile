# chant — recipe cache for coding agents.
.DEFAULT_GOAL := build
BIN := bin/chant

.PHONY: build test bench fmt vet check install clean

build: ## build the chant binary into bin/
	go build -o $(BIN) ./cmd/chant

test: ## run the Go test suite
	go test ./... -count=1

bench: build ## run the recipe-engine validation suite
	./$(BIN) bench

fmt: ## format the Go sources
	gofmt -w cmd internal

vet: ## go vet
	go vet ./...

check: fmt vet test bench ## fmt + vet + test + bench (pre-commit sanity)

install: ## install chant to $GOBIN / $HOME/go/bin
	go install ./cmd/chant

clean: ## remove build artifacts
	rm -rf bin dist

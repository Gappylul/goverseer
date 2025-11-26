.PHONY: test
test:
	go test -v -race -cover ./...

.PHONY: test-coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: bench
bench:
	go test -bench=. -benchmem ./...

.PHONY: fmt
fmt:
	gofmt -s -w .

.PHONY: vet
vet:
	go vet ./...

.PHONY: build
build:
	go build -v ./...

.PHONY: clean
clean:
	rm -f coverage.out coverage.html
	go clean -testcache

.PHONY: mod-tidy
mod-tidy:
	go mod tidy

.PHONY: examples
examples:
	@echo "Running basic example..."
	cd examples/basic && go run main.go &
	sleep 3
	pkill -f "examples/basic"
	@echo "\nRunning restart types example..."
	cd examples/restart_types && go run main.go

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  test           - Run tests with race detector"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  bench          - Run benchmarks"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  build          - Build package"
	@echo "  clean          - Clean build artifacts"
	@echo "  mod-tidy       - Tidy go.mod"
	@echo "  examples       - Run example programs"
.PHONY: build test test-integration vet coverage clean build-all release

BINARY=ilnamiqui
VERSION?=dev

build:
	go build -ldflags="-X github.com/beabys/ilnamiqui/internal/cli.version=$(VERSION)" -o $(BINARY) ./cmd/cli/

test:
	go test ./... -count=1

test-integration:
	go test -tags=integration -count=1 ./...

vet:
	go vet ./...

coverage:
	@echo "Generating coverage report (skipping internal/mocks)..."
	@go list ./... | grep -v internal/mocks | xargs go test -coverprofile=coverage.out -covermode=atomic -count=1
	@go tool cover -func=coverage.out | tail -1
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

build-all:
	./scripts/build.sh $(VERSION)

clean:
	rm -rf $(BINARY) .ilnamiqui/

release:
	@echo "Tag and push to trigger GitHub Release:"
	@echo "  git tag v$(VERSION) && git push origin v$(VERSION)"

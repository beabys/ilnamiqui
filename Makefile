.PHONY: build test test-integration vet clean build-all release

BINARY=ilnamiqui
VERSION?=dev

build:
	go build -ldflags="-X github.com/beabys/ilnamiqui/internal/cli.version=$(VERSION)" -o $(BINARY) ./cmd/ilnamiqui/

test:
	go test ./... -count=1

test-integration:
	go test -tags=integration -count=1 ./...

vet:
	go vet ./...

build-all:
	./scripts/build.sh $(VERSION)

clean:
	rm -rf $(BINARY) .ilnamiqui/

release:
	@echo "Tag and push to trigger GitHub Release:"
	@echo "  git tag v$(VERSION) && git push origin v$(VERSION)"

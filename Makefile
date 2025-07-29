.PHONY: swagger build run test clean dev release docker-build dev-deps generate lint frontend-build

# Build variables
BINARY_NAME=marmot
GO_FILES=$(shell find . -name '*.go')
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || echo "v0.0.0")
LDFLAGS_VERSION=-X "github.com/marmotdata/marmot/cmd.Version=$(VERSION)"

swagger:
	swag init -d internal/api/v1 --generalInfo server.go --parseDependency --output docs

build:
	go build -o bin/$(BINARY_NAME) cmd/main.go

dev: swagger build
	./bin/$(BINARY_NAME) run

frontend-build:
	cd web/marmot && pnpm install && pnpm build
	mkdir -p internal/staticfiles/build
	cp -r web/marmot/build/* internal/staticfiles/build/

release: clean swagger frontend-build
	go build -tags=production -ldflags '$(LDFLAGS_VERSION)' -o bin/$(BINARY_NAME) cmd/main.go
	rm -rf internal/staticfiles/build

test:
	go test -v ./...

e2e-test: build test e2e-client
	cd test/e2e && go test -v -timeout 1h ./...

clean:
	rm -rf bin/ internal/static/build/
	go clean

generate:
	# Cleanup old docs before generating
	find web/docs/docs/Plugins -type f ! -name "index.md" ! -name "_category_.json" -delete
	find web/docs/docs/Plugins -type d -empty -delete
	go generate ./...

lint:
	golangci-lint run --config=./.github/.golangci.yaml ./... -v

docker-build:
	docker build -t marmot -f deployments/docker/Dockerfile.backend .

dev-deps:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.60.1
	go install github.com/swaggo/swag/cmd/swag@latest

e2e-client: swagger
	rm -rf test/e2e/internal/client/*
	cd test/e2e && swagger generate client -f ../../docs/swagger.yaml -A marmot --target internal/client

chart-test:
	docker run ${DOCKER_ARGS} --user root --entrypoint /bin/sh --rm -v $(CURDIR):/charts -w /charts helmunittest/helm-unittest:3.17.3-0.8.2 /charts/.github/test.sh

chart-lint:
	docker run ${DOCKER_ARGS} --env GIT_SAFE_DIR="true" --entrypoint /bin/sh --rm -v $(CURDIR):/charts -w /charts quay.io/helmpack/chart-testing:v3.13.0 /charts/.github/lint.sh

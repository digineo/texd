MAKEFLAGS += --no-print-directory

TARGET = texd

VERSION     = $(shell git describe --tags --always --dirty)
COMMIT_DATE = $(shell git show -s --format=%cI HEAD)
BUILD_DATE  = $(shell date --iso-8601=seconds)
DEVELOPMENT = 1

LDFLAGS = -s -w \
          -X 'github.com/digineo/texd.version=$(VERSION)' \
          -X 'github.com/digineo/texd.commitat=$(COMMIT_DATE)' \
          -X 'github.com/digineo/texd.buildat=$(BUILD_DATE)' \
          -X 'github.com/digineo/texd.isdev=$(DEVELOPMENT)'
GOFLAGS = -trimpath -ldflags="$(LDFLAGS)"

# passed to run-* targets
RUN_ARGS = --job-directory ./tmp --log-level debug

## help (prints target names with trailing "## comment")

PHONY: help
help: ## print a short help message
	@grep -hE '^[a-zA-Z_-]+:[^:]*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

## building

build: $(TARGET) ## build a development binary

.PHONY: $(TARGET)
$(TARGET):
	go build -o $@ $(GOFLAGS) ./cmd/texd

.PHONY: clean
clean: ## cleanup build fragments
	rm -rf tmp/ dist/ texd coverage.*


## development

.PHONY: lint
lint: ## runs golangci-lint on source files
	golangci-lint run

.PHONY: run-local
run-local: tmp build ## builds and runs texd in local mode
	./$(TARGET) $(RUN_ARGS)

.PHONY: run-container
run-container: tmp build ## builds and runs texd in container mode
	./$(TARGET) $(RUN_ARGS) texlive/texlive:latest


## testing

.PHONY: coverage.out
coverage.out:
	go test -race -covermode=atomic -coverprofile=$@ ./...

coverage.html: coverage.out
	go tool cover -html $< -o $@

.PHONY: test
test: coverage.out ## runs tests

.PHONY: test-simple
test-simple: tmp ## sends a simple document to a running instance
	curl http://localhost:2201/render \
		-F "input.tex=<testdata/simple/input.tex" \
		-s -o tmp/$@-$$(date +%F_%T)-$$$$

.PHONY: test-multi
test-multi: tmp ## sends a more complex document to a running instance
	curl http://localhost:2201/render \
		-F "input.tex=<testdata/multi/input.tex" \
		-s -F "doc.tex=<testdata/multi/doc.tex" \
		-F "chapter/input.tex=<testdata/multi/chapter/input.tex" \
		-o tmp/$@-$$(date +%F_%T)-$$$$

.PHONY: test-missing
test-missing: tmp ## send a broken document to a running instance
	curl 'http://localhost:2201/render?errors=condensed' \
		-F "input.tex=<testdata/missing/input.tex" \
		-s -o tmp/$@-$$(date +%F_%T)-$$$$

.PHONY: test-load
test-load: tmp ## sends 200 documents to a running instance
	for i in $$(seq 1 100); do \
		$(MAKE) -j2 test-multi test-missing & \
	done

## release engineering

.PHONY: release-test
release-test: ## runs goreleaser, but skips publishing
	goreleaser release --rm-dist --skip-publish

.PHONY: release-publish
release-publish: ## runs goreleaser and publishes artifacts
	goreleaser release --rm-dist

.PHONY: docker-latest
docker-latest: build ## builds a Docker container with the latest binary
	docker build --pull \
		--label=org.opencontainers.image.created=$(BUILD_DATE) \
		--label=org.opencontainers.image.title=$(TARGET) \
		--label=org.opencontainers.image.revision=$(shell git show -s --format=%H HEAD) \
		--label=org.opencontainers.image.version=$(VERSION) \
		--platform=linux/amd64 \
		-t digineode/texd:latest \
		.

.PHONY: bump bump-major bump-minor bump-patch
bump: bump-patch ## bump version
bump-major: ## bump major version
	go run ./cmd/build bump --major
bump-minor: ## bump minor version
	go run ./cmd/build bump --minor
bump-patch: ## bump patch version
	go run ./cmd/build bump

## misc

tmp:
	mkdir -p ./tmp

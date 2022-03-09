GOFLAGS = -trimpath -ldflags="-s -w"

build: texd

.PHONY: texd
texd:
	go build -o $@ $(GOFLAGS) ./cmd/texd

.PHONY: run
run:
	mkdir -p ./tmp
	go run $(GOFLAGS) ./cmd/texd -D ./tmp

.PHONY: test
test:
	go test -race ./...

.PHONY: test-simple
test-simple:
	curl http://localhost:2201/render \
		-F "input.tex=<testdata/simple/input.tex"

.PHONY: test-multi
test-multi:
	curl http://localhost:2201/render \
		-F "input.tex=<testdata/multi/input.tex" \
		-F "doc.tex=<testdata/multi/doc.tex" \
		-F "chapter/input.tex=<testdata/multi/input.tex"

.PHONY: texlive-2020-image
texlive-2020-image:
	docker build --pull --rm \
		-f docker/Dockerfile.texlive2020 \
		--tag texd-texlive2020 \
		docker

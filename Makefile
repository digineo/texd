MAKEFLAGS += --no-print-directory

TARGET  = texd
GOFLAGS = -trimpath -ldflags="-s -w"

## building

build: $(TARGET)

.PHONY: $(TARGET)
$(TARGET):
	go build -o $@ $(GOFLAGS) ./cmd/texd

.PHONY: clean
clean:
	rm -rf tmp texd


## development

.PHONY: run-local
run-local: tmp build
	./$(TARGET) -D ./tmp

.PHONY: run-container
run-container: tmp build
	./$(TARGET) -D ./tmp texlive/texlive:latest


## testing

.PHONY: test
test:
	go test -race ./...

.PHONY: test-simple
test-simple: tmp
	curl http://localhost:2201/render \
		-F "input.tex=<testdata/simple/input.tex" \
		-s -o tmp/$@-$$(date +%F_%T)-$$$$

.PHONY: test-multi
test-multi: tmp
	curl http://localhost:2201/render \
		-F "input.tex=<testdata/multi/input.tex" \
		-s -F "doc.tex=<testdata/multi/doc.tex" \
		-F "chapter/input.tex=<testdata/multi/chapter/input.tex" \
		-o tmp/$@-$$(date +%F_%T)-$$$$

.PHONY: test-missing
test-missing: tmp
	curl 'http://localhost:2201/render?errors=condensed' \
		-F "input.tex=<testdata/missing/input.tex" \
		-s -o tmp/$@-$$(date +%F_%T)-$$$$

.PHONY: test-load
test-load: tmp
	for i in $$(seq 1 100); do \
		$(MAKE) test-missing & \
	done

## misc

.PHONY: texlive-2020-image
texlive-2020-image:
	docker build --pull --rm \
		-f docker/Dockerfile.texlive2020 \
		--tag texd-texlive2020 \
		docker

tmp:
	mkdir -p ./tmp

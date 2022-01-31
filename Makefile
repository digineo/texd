.PHONY: run
run:
	cd cmd/texd && go run main.go

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

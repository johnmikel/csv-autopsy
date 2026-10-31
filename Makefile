# csv-autopsy: find the damage spreadsheets do to CSV files.
GLUE := $(shell go env GOROOT)/lib/wasm/wasm_exec.js

.PHONY: all test build page sample clean

all: test build page

test:
	go vet ./...
	go test ./...

build:
	go build -o bin/csv-autopsy .

# The web page: the checker compiled to WebAssembly, inlined with the sample file.
page:
	mkdir -p dist
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o dist/csv-autopsy.wasm ./cmd/wasm
	go run ./cmd/bundle -glue "$(GLUE)"

# The sample files, regenerated from testdata/generate.go.
sample:
	go run testdata/generate.go

clean:
	rm -rf bin dist

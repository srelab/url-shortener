PACKAGES ?= $(shell go list ./... | grep -v /vendor/)
GOFILES := $(shell find . -name "*.go" -type f -not -path "./vendor/*")
GOFMT ?= gofmt "-s"
BUILD ?= go build -o ./url-shortener cmd/url-shortener/main.go
PACK ?= gzip ./url-shortener

fmt:
	$(GOFMT) -w $(GOFILES)

vet:
	go vet $(PACKAGES)

.PHONY: build
build:
	$(BUILD)

.PHONY: pack
pack:
	$(PACK)

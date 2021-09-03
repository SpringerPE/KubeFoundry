#!/usr/bin/make -f

PROJECTNAME := $(shell basename "$(PWD)")
BINARY := ${PROJECTNAME}
BUILD := $(shell git rev-parse --short HEAD)
VERSION := $(shell head -n1 VERSION)
CHANGES := $(shell test -n "$$(git status --porcelain)" && echo '+CHANGES' || true)
PKGS := $(shell go list ./... | grep -v /vendor)
LDFLAGS := -X main.Build=$(BUILD) -X main.Version=$(VERSION)

# Go mod
export GO111MODULE=on

# Define architectures
BUILDER := linux-amd64 linux-arm64 darwin-amd64 windows-amd64

# Go paths and tools
GOBIN := $(GOPATH)/bin
GOCMD := go
GOVET := $(GOCMD) tool vet
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOLINT := $(GOBIN)/golint
ERRCHECK := $(GOBIN)/errcheck
STATICCHECK := $(GOBIN)/staticcheck

.PHONY: help
help:	### Show targets documentation
ifeq ($(UNAME), Linux)
	@grep -P '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
else
	@awk -F ':.*###' '$$0 ~ FS {printf "%15s%s\n", $$1 ":", $$2}' \
		$(MAKEFILE_LIST) | grep -v '@awk' | sort
endif

.PHONY: all
all: test build  ### Test and build the binaries

.PHONY: clean-all
clean-all: clean clean-vendor clean-build  ### Clean all artifacts, packages and vendor dependencies

.PHONY: clean
clean:  ### Delete go resources
	@echo "*** Deleting go resources ***"
	$(GOCLEAN) -i ./...

.PHONY: clean-vendor
clean-vendor:  ### Delete vendor packages
	@echo "*** Deleting vendor packages ***"
	find $(CURDIR)/vendor -type d -print0 2>/dev/null | xargs -0 rm -Rf

.PHONY: clean-build
clean-build:  ### Delete builds and OS packages
	@echo "*** Deleting builds ***"
	@rm -Rf build/*
	@rm -Rf deb/*

.PHONY: test
test:  ### Run golang tests
	@echo "*** Running tests ***"
	$(GOTEST) -v ./...

.PHONY: lint
lint: golint vet errcheck staticcheck unused checklicense  ### Run all linting checks

$(GOLINT):
	go get -u -v github.com/golang/lint/golint

$(ERRCHECK):
	go get -u github.com/kisielk/errcheck

$(STATICCHECK):
	go get -u honnef.co/go/tools/cmd/staticcheck

$(UNUSED):
	go get -u honnef.co/go/tools/cmd/unused

.PHONY: golint
golint: $(GOLINT)  ### Run golint
	$(GOLINT) $(PKGS)

.PHONY: vet
vet: ### Run vet
	$(GOVET) -v $(PKGS)

.PHONY: errcheck
errcheck: $(ERRCHECK)  ### Run errcheck
	$(ERRCHECK) ./...

.PHONY: staticcheck
staticcheck: $(STATICCHECK)  ### Run staticcheck
	$(STATICCHECK) ./...

.PHONY: unused
unused: $(UNUSED)  ### Run unused
	$(UNUSED) ./...

.PHONY: $(GOMETALINTER)
$(GOMETALINTER): ### Run gometalinter
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install &> /dev/null

# linux-amd64, linux-arm-6, linux-arm-7, linux-arm64
.PHONY: $(BUILDER)
$(BUILDER):  ### Build specific binary
	@echo "*** Building binary for $@ ***"
	$(eval OS := $(word 1,$(subst -, ,$@)))
	$(eval OSARCH := $(word 2,$(subst -, ,$@)))
	$(eval ARCHV := $(word 3,$(subst -, ,$@)))
	@mkdir -p build
	GOOS=${OS} GOARCH=${OSARCH} ${GOBUILD} -ldflags "${LDFLAGS}" -o build/${BINARY}-${VERSION}-${OS}-${OSARCH}${ARCHV}

# from all
.PHONY: build
build: $(BUILDER)  ### Build binaries

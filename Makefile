GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

.PHONY: setup

setup:
	go install github.com/google/yamlfmt/cmd/yamlfmt@latest
	go install github.com/rhysd/actionlint/cmd/actionlint@latest
	go install github.com/evilmartians/lefthook@latest
	$(GOBIN)/lefthook install

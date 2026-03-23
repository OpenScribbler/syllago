.PHONY: build clean build-all test fmt vet setup

build:
	$(MAKE) -C cli build

clean:
	$(MAKE) -C cli clean

build-all:
	$(MAKE) -C cli build-all

test:
	$(MAKE) -C cli test

fmt:
	$(MAKE) -C cli fmt

vet:
	$(MAKE) -C cli vet

setup:
	git config core.hooksPath .githooks
	@echo "Git hooks configured (.githooks/)"

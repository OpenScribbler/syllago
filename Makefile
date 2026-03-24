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
	@# Also install to .git/hooks/ as fallback — bd (beads) may reset core.hooksPath
	@cp -f .githooks/pre-commit .git/hooks/pre-commit 2>/dev/null || true
	@cp -f .githooks/pre-push .git/hooks/pre-push 2>/dev/null || true
	@chmod +x .git/hooks/pre-commit .git/hooks/pre-push 2>/dev/null || true
	@echo "Git hooks configured (.githooks/ + .git/hooks/ fallback)"

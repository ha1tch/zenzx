# zenzx - Makefile front-end
#
# Two binaries, deliberately named without OS/arch suffixes -- those belong
# on release archives (see `make release`), never on the binaries
# themselves:
#   * zenzx           the GUI (raylib + oto). Needs cgo on Linux (oto's ALSA
#                      backend); cgo-free (purego) on macOS and Windows.
#   * zenzx-headless   no window, no audio device, renders to PNG. Always
#                      cgo-free, cross-compiles cleanly to every platform.
#
# Every build/test invocation here matches what .github/workflows/ci.yml
# and release.yml actually run -- verified directly, not just written to
# look right (see CHANGELOG for the two real CI bugs that verification
# caught: a missing CGO_ENABLED=0 on one test line, and six go-vet
# failures that would have blocked the native Linux GUI build).

VERSION := $(shell tr -d ' \t\r\n' < VERSION)
DIST    := dist

GO      ?= go

.DEFAULT_GOAL := help

## help: show this help
.PHONY: help
help:
	@echo "zenzx v$(VERSION) - targets:"
	@echo
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
	@echo

# =============================================================================
# Build
# =============================================================================

## build: build both binaries (zenzx, zenzx-headless) into $(DIST)
.PHONY: build
build: build-gui build-headless

## build-gui: build the GUI binary (cgo; needs system GL/X11/Wayland/ALSA dev libs on Linux)
.PHONY: build-gui
build-gui:
	@mkdir -p $(DIST)
	CGO_ENABLED=1 $(GO) build -ldflags "-s -w -X main.version=$(VERSION)" -o $(DIST)/zenzx .
	@echo "built $(DIST)/zenzx"

## build-gui-purego: build the GUI binary cgo-free (works on this host only if oto needs no cgo here -- i.e. not native Linux)
.PHONY: build-gui-purego
build-gui-purego:
	@mkdir -p $(DIST)
	CGO_ENABLED=0 $(GO) build -tags purego -ldflags "-s -w -X main.version=$(VERSION)" -o $(DIST)/zenzx .
	@echo "built $(DIST)/zenzx (purego)"

## build-headless: build the headless binary (always cgo-free)
.PHONY: build-headless
build-headless:
	bash build_headless.sh $(DIST)/zenzx-headless

## cross: verify cross-compilation for every release target (to /dev/null -- see `make release` for real artifacts)
.PHONY: cross
cross:
	@echo "-- headless (cgo-free, all platforms) --"
	@for p in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64; do \
		os=$${p%/*}; arch=$${p#*/}; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build -tags headless -o /dev/null . && echo "ok  headless  $$p"; \
	done
	@echo "-- gui, cgo-free purego targets --"
	@for p in darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do \
		os=$${p%/*}; arch=$${p#*/}; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build -o /dev/null . && echo "ok  gui       $$p"; \
	done
	@echo "-- gui, native cgo (this host only) --"
	@CGO_ENABLED=1 $(GO) build -o /dev/null . && echo "ok  gui       $$(go env GOOS)/$$(go env GOARCH) (cgo)"

# =============================================================================
# Testing
# =============================================================================
#
# CGO_ENABLED=0 on the ./pkg/... line matters: pkg/zenuiraylib imports
# raylib directly and unconditionally (no build constraint of its own), so
# without it this pulls in raylib's cgo path and fails on any machine
# missing the full GL/X11/Wayland dev headers -- confirmed as a real CI
# failure, not a theoretical one, before this was added.

## test: run the full test suite (both scopes, cgo-free)
.PHONY: test
test:
	CGO_ENABLED=0 $(GO) test -count=1 ./pkg/...
	CGO_ENABLED=0 $(GO) test -tags headless -count=1 .

## test-race: run the test suite under the race detector
.PHONY: test-race
test-race:
	CGO_ENABLED=0 $(GO) test -race -count=1 ./pkg/...
	CGO_ENABLED=0 $(GO) test -race -tags headless -count=1 .

## vet: vet both scopes (headless cgo-free, GUI native cgo)
.PHONY: vet
vet:
	CGO_ENABLED=0 $(GO) vet -tags headless ./...
	$(GO) vet .

## smoke: boot-test the headless binary (requires build-headless first)
.PHONY: smoke
smoke: build-headless
	cp $(DIST)/zenzx-headless .
	bash smoke_headless.sh
	rm -f zenzx-headless

# =============================================================================
# Formatting and linting
# =============================================================================

## fmt: gofmt the whole tree in place
.PHONY: fmt
fmt:
	gofmt -w .

## fmt-check: fail if anything is not gofmt-clean
.PHONY: fmt-check
fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt would change:"; echo "$$out"; exit 1; fi
	@echo "gofmt clean"

## lint: run golangci-lint (see .golangci.yml)
.PHONY: lint
lint:
	golangci-lint run ./...

# =============================================================================
# Release gates (mirrors .repoman.json's own release pipeline steps)
# =============================================================================

## check-changelog: verify CHANGELOG.md has an entry for the current VERSION
.PHONY: check-changelog
check-changelog:
	sh ./check_changelog.sh

## check-gui: type-check the GUI sources (links if system libs are present)
.PHONY: check-gui
check-gui:
	sh ./check_gui.sh

## check-register: verify the work register is internally consistent
.PHONY: check-register
check-register:
	python3 repoman/register.py check

## check-gomod: verify go.mod carries no local-path replace directives
.PHONY: check-gomod
check-gomod:
	python3 repoman/gomod.py check

## sync: propagate VERSION into pkg/version and the tracking docs
.PHONY: sync
sync:
	python3 repoman/syncver.py set $(VERSION)

## check: the full local pre-commit gate
.PHONY: check
check: fmt-check vet check-gomod check-register cross test
	@echo "✓ all checks passed"

## release: run the full repoman release pipeline (usage: make release RELEASE_VERSION=0.5.0)
.PHONY: release
release:
	@if [ -z "$(RELEASE_VERSION)" ]; then echo "Usage: make release RELEASE_VERSION=<version>"; exit 1; fi
	python3 repoman/relcore.py $(RELEASE_VERSION)

## release-resume: resume an interrupted release
.PHONY: release-resume
release-resume:
	@if [ -z "$(RELEASE_VERSION)" ]; then echo "Usage: make release-resume RELEASE_VERSION=<version>"; exit 1; fi
	python3 repoman/relcore.py $(RELEASE_VERSION) --resume

# =============================================================================
# Documentation
# =============================================================================

## testing-md: regenerate TESTING.md from the current test run
.PHONY: testing-md
testing-md:
	python3 scripts/gen_testing_md.py

# =============================================================================
# Misc
# =============================================================================

## tidy: go mod tidy
.PHONY: tidy
tidy:
	$(GO) mod tidy

## clean: remove build artifacts
.PHONY: clean
clean:
	rm -rf $(DIST)
	rm -f zenzx zenzx-headless
	@echo "cleaned"

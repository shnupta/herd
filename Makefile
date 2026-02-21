.PHONY: build install clean

# Get version from git describe (e.g., "1.5" or "1.5-3-gabc1234")
GIT_DESC := $(shell git describe --tags 2>/dev/null)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

ifdef GIT_DESC
    # Check if exactly on a tag
    GIT_EXACT := $(shell git describe --tags --exact-match 2>/dev/null)
    ifdef GIT_EXACT
        # Exact tag match (release build)
        BUILD_VERSION := $(GIT_DESC)
    else
        # Ahead of tag: 1.5-3-gabc1234 -> 1.5-dev+abc1234
        BASE_VERSION := $(shell echo $(GIT_DESC) | sed 's/-.*//')
        BUILD_VERSION := $(BASE_VERSION)-dev+$(GIT_COMMIT)
    endif
else
    # No tags yet
    BUILD_VERSION := 0.0-dev+$(GIT_COMMIT)
endif

LDFLAGS := -s -w -X main.version=$(BUILD_VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o herd .

install:
	go install -ldflags "$(LDFLAGS)" .

clean:
	rm -f herd

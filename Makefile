VERSION  ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "dev")
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DIRTY    ?= $(shell git diff-index --quiet HEAD -- 2>/dev/null || echo "+dirty")
TIMESTAMP?= $(shell date -u +"%Y-%m-%dT%H:%M:%S")

LDFLAGS := -s -w \
	-X smith/cmd.version=$(VERSION)$(DIRTY) \
	-X smith/cmd.timestamp=$(TIMESTAMP) \
	-X smith/cmd.commit=$(COMMIT)

.PHONY: build clean version

build:
	go build -ldflags "$(LDFLAGS)" -o smith .

clean:
	rm -f smith

version:
	@echo "Version: $(VERSION)$(DIRTY)"
	@echo "Commit:  $(COMMIT)"
	@echo "Time:    $(TIMESTAMP)"

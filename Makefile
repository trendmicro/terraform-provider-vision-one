.PHONY: build clean
.PHONY: lint lint-fix

# Get golangci-lint binary path
GOPATH=$(shell go env GOPATH)
GOBIN=$(shell go env GOBIN)
ifeq ($(GOBIN),)
	LINT_BINARY_PATH=$(GOPATH)/bin/golangci-lint
else
	LINT_BINARY_PATH=$(GOBIN)/golangci-lint
endif

# Get all function binaries for this code base
# Find all directories with .go files
TARGETS=$(sort $(dir $(wildcard services/public/func/*/*.go)))
HANDLERS=$(addsuffix bootstrap,$(TARGETS))
PACKAGES=$(HANDLERS:/bootstrap=.zip)
ARTIFACT=bin/

build: setup test $(ARTIFACT) $(HANDLERS) $(PACKAGES)

%/bootstrap: %/*.go
	env GOARCH=amd64 GOOS=linux go build -tags lambda.norpc -o $@ ./$*
	zip -FS -j $*.zip $@
	cp $*.zip $(ARTIFACT)

$(ARTIFACT):
	@mkdir -p $(dir $(ARTIFACT))

tidy: | node_modules/go.mod
	go mod tidy

test:
	go test -tags "testtools" -v ./... -coverprofile=coverage.out

coverage:
	go tool cover -html=coverage.out

# node_modules/go.mod used to ignore possible go modules in node_modules.
node_modules/go.mod:
	-@touch $@

vars:
	@echo TARGETS: $(TARGETS)
	@echo HANDLERS: $(HANDLERS)
	@echo PACKAGES: $(PACKAGES)

setup:
	@echo "Checking golangci-lint for building..."
	@if [ ! -e "$(LINT_BINARY_PATH)" ]; then \
		echo "golangci-lint is not installed. Installing..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; \
	else \
			echo "golangci-lint is already installed."; \
	fi

lint: setup
	@echo "Running golangci-lint..."
	$(LINT_BINARY_PATH) run -v ./... --config ./.golangci.yml

lint-fix: setup ## Run golangci-lint and prettier formatting fixers and go mod tidy
	@echo "Running golangci-lint auto-fix..."
	$(LINT_BINARY_PATH) run -v ./... --fix --config ./.golangci.yml
	go mod tidy

clean:
	$(RM) $(HANDLERS) $(PACKAGES)
	$(RM) -r $(ARTIFACT)
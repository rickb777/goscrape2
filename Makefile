GOLANGCI_VERSION = v1.60.3

default: test
	go vet ./...
	go build -o goscrape2 -ldflags "-s -X main.version=`./.version.sh` -X main.date=`date '+%F'`" .

help: ## show help
	@grep -E '^[0-9a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

test: ## run tests
	@rm -f goscrape2
	go test -timeout 10s -race ./...

test-coverage: ## run unit tests and create test coverage
	go test -timeout 10s ./... -coverprofile .testCoverage -covermode=atomic -coverpkg=./...

test-coverage-web: test-coverage ## run unit tests and show test coverage in browser
	go tool cover -func .testCoverage | grep total | awk '{print "Total coverage: "$$3}'
	go tool cover -html=.testCoverage

install: ## install all binaries
	go install -buildvcs=false -ldflags "-s -X main.version=`./.version.sh` -X main.date=`date '+%F'`" .

release-snapshot: ## build release binaries from current git state as snapshot
	goreleaser release --snapshot --clean

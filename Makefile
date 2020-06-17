.DEFAULT_GOAL := help

.PHONY: setup
setup: ## Resolve dependencies using Go Modules
	GO111MODULE=on go mod download

.PHONY: test
test: ## Tests all code
	GO111MODULE=on go test -cover -race ./...

.PHONY: lint
lint: ## Runs static code analysis
	command -v golint >/dev/null 2>&1 || { GO111MODULE=on go get -u golang.org/x/lint/golint; }
	go list ./... | xargs -L1 golint -set_exit_status
	npm run lint

.PHONY: run
run: ## Run web application locally
	GO111MODULE=on go run `find cmd/server -type f -not -name "*_test.go" | tr '\r\n' ' '`

.PHONY: docker-build
docker-build: ## Build a docker image
	docker build -t kaginawa/kaginawa-server .

.PHONY: docker-run
docker-run: ## Run a builded docker image using "docker-env.txt" (list of KEY=VALUE)
	if test -e docker-env.txt; \
	then docker run --env-file=docker-env.txt -p 8080:80 -t kaginawa/kaginawa-server; \
	else docker run -p 8080:80 -t kaginawa/kaginawa-server; \
	fi

.PHONY: count-go
count-go: ## Count number of lines of all go codes
	find . -name "*.go" -type f | xargs wc -l | tail -n 1

.PHONY: count-html
count-html: ## Count number of lines of all go html templates
	find . -name "*.html" -type f | xargs wc -l | tail -n 1

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

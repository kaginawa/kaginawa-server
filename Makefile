.DEFAULT_GOAL := help

.PHONY: setup
setup: ## Resolve dependencies using Go Modules
	go mod download

.PHONY: test
test: ## Tests all code
	go test -cover -race ./...

.PHONY: lint
lint: ## Runs static code analysis
	command -v golint >/dev/null 2>&1 || { GO111MODULE=off go get -u golang.org/x/lint/golint; }
	golint -set_exit_status ./...
	npm run lint

.PHONY: run
run: ## Run web application locally
	go run ./cmd/server

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
	find . -name "*.gohtml" -type f | xargs wc -l | tail -n 1

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

APP_NAME=tezos-delegation-service

.PHONY: help run test test-integration lint docker-build docker-up docker-down db-up db-down

.DEFAULT_GOAL := help

help:
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

run: ## Run the application locally
	go run ./cmd

test: ## Run unit tests (skips integration tests)
	go test -short ./...

test-integration: ## Run all tests including integration tests (requires database)
	go test ./...

docker-build: ## Build Docker image
	docker build -t $(APP_NAME):latest .

docker-up: ## Start all services with Docker Compose
	docker-compose up --build

docker-down: ## Stop all Docker Compose services
	docker-compose down

db-up: ## Start only the PostgreSQL database
	docker-compose up -d db

db-down: ## Stop all Docker Compose services
	docker-compose down

lint: ## Run linter 
	@echo "golangci-lint not configured"

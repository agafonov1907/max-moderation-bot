include .env
export

.PHONY: build run db-start db-stop db-clean lint test

# Build the application
build:
	go build -o bin/bot ./cmd/bot/main.go

# Run the application (build and run in one step)
run:
	go build -o bin/bot ./cmd/bot/main.go && ./bin/bot

# Start the database
db-start:
	docker-compose up -d db

# Stop the database
db-stop:
	docker-compose stop db

# Stop the database and remove volumes (clean state)
db-clean:
	docker-compose stop db && docker-compose down -v



# Migrations
GOOSE_DRIVER=postgres
DATABASE_URL=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable
GOOSE_DBSTRING=$(DATABASE_URL)
MIGRATIONS_DIR=./migrations

migrate-up:
	GOOSE_DRIVER=$(GOOSE_DRIVER) GOOSE_DBSTRING="$(GOOSE_DBSTRING)" go run github.com/pressly/goose/v3/cmd/goose -dir $(MIGRATIONS_DIR) up

migrate-down:
	GOOSE_DRIVER=$(GOOSE_DRIVER) GOOSE_DBSTRING="$(GOOSE_DBSTRING)" go run github.com/pressly/goose/v3/cmd/goose -dir $(MIGRATIONS_DIR) down

migrate-create:
	GOOSE_DRIVER=$(GOOSE_DRIVER) GOOSE_DBSTRING="$(GOOSE_DBSTRING)" go run github.com/pressly/goose/v3/cmd/goose -dir $(MIGRATIONS_DIR) create $(NAME) sql

# Run tests
test:
	go test ./...

# Generate architecture diagram
diagram:
	npx -y -p @mermaid-js/mermaid-cli mmdc -i assets/architecture.mmd -o assets/architecture.svg

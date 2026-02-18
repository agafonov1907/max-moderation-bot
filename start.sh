#!/bin/sh
set -e

# Construct database connection string from environment variables
export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"

echo "Running database migrations..."
goose -dir ./migrations up

echo "Starting application..."
exec ./bot

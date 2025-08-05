#!/bin/bash

# Database setup script for SSVirt
# This script creates the PostgreSQL database and user for development

set -e

DB_NAME="${DB_NAME:-ssvirt}"
DB_USER="${DB_USER:-ssvirt}"
DB_PASSWORD="${DB_PASSWORD:-ssvirt_dev_password}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-}"

echo "Setting up SSVirt database..."

# Set PGPASSWORD environment variable to avoid password exposure in process list
export PGPASSWORD="$POSTGRES_PASSWORD"

# Create database and user
psql -h "$DB_HOST" -p "$DB_PORT" -U "$POSTGRES_USER" -c "CREATE DATABASE $DB_NAME;" || echo "Database $DB_NAME already exists"
psql -h "$DB_HOST" -p "$DB_PORT" -U "$POSTGRES_USER" -c "CREATE USER $DB_USER WITH ENCRYPTED PASSWORD '$DB_PASSWORD';" || echo "User $DB_USER already exists"
psql -h "$DB_HOST" -p "$DB_PORT" -U "$POSTGRES_USER" -c "GRANT ALL PRIVILEGES ON DATABASE $DB_NAME TO $DB_USER;"
psql -h "$DB_HOST" -p "$DB_PORT" -U "$POSTGRES_USER" -d "$DB_NAME" -c "GRANT ALL ON SCHEMA public TO $DB_USER;"

# Create UUID extension
psql -h "$DB_HOST" -p "$DB_PORT" -U "$POSTGRES_USER" -d "$DB_NAME" -c "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";"

# Clear the password from environment for security
unset PGPASSWORD

echo "Database setup completed successfully!"
echo "Database URL: postgresql://$DB_USER:$DB_PASSWORD@$DB_HOST:$DB_PORT/$DB_NAME"
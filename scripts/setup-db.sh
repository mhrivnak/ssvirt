#!/bin/bash

# Database setup script for SSVirt
# This script creates the PostgreSQL database and user for development

set -e

# Function to generate a secure random password
generate_password() {
    # Generate a 24-character password with alphanumeric characters and safe symbols
    # Using /dev/urandom for cryptographically secure randomness
    if command -v openssl >/dev/null 2>&1; then
        # Use openssl if available (most secure)
        openssl rand -base64 18 | tr -d '/+=' | head -c 24
    elif [[ -r /dev/urandom ]]; then
        # Fallback to /dev/urandom with tr
        LC_ALL=C tr -dc 'A-Za-z0-9!@#$%^&*' </dev/urandom | head -c 24
    else
        # Last resort fallback using date and random
        echo "$(date +%s)$(shuf -i 1000-9999 -n 1)abcXYZ" | head -c 24
    fi
}

DB_NAME="${DB_NAME:-ssvirt}"
DB_USER="${DB_USER:-ssvirt}"
# Generate secure random password if not provided via environment
PASSWORD_FILE=".ssvirt-db-password"
if [[ -z "${DB_PASSWORD:-}" ]]; then
    # Check if password file exists and use it
    if [[ -f "$PASSWORD_FILE" ]]; then
        DB_PASSWORD="$(grep SSVIRT_DB_PASSWORD "$PASSWORD_FILE" | cut -d= -f2)"
        echo "Using existing password from $PASSWORD_FILE"
    else
        # Generate new password and save it
        DB_PASSWORD="$(generate_password)"
        echo "Generated secure random password for database user $DB_USER"
        
        # Save password to secure file for future use
        echo "SSVIRT_DB_PASSWORD=$DB_PASSWORD" > "$PASSWORD_FILE"
        chmod 600 "$PASSWORD_FILE"
        echo "Password saved to $PASSWORD_FILE (readable only by you)"
        echo "To reuse this password, run: export SSVIRT_DB_PASSWORD=\$(grep SSVIRT_DB_PASSWORD $PASSWORD_FILE | cut -d= -f2)"
    fi
else
    DB_PASSWORD="${DB_PASSWORD}"
fi
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
echo "Database URL: postgresql://$DB_USER:****@$DB_HOST:$DB_PORT/$DB_NAME"
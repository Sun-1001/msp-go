#!/bin/bash
# Initialize PostgreSQL extensions required by the Go backend.

set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE EXTENSION IF NOT EXISTS vector;
    SELECT extname, extversion FROM pg_extension WHERE extname = 'vector';
EOSQL

echo "pgvector extension enabled"

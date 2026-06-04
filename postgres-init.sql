-- PostgreSQL initialization script for HelixCode
-- Creates the required database, extensions, and privileges.
--
-- NOTE (CONST-042 / Article XII §12.1 — No-Secret-Leak):
-- The "helix" superuser is created by the official postgres image's own
-- entrypoint from the POSTGRES_USER / POSTGRES_PASSWORD environment variables
-- (sourced from .env, mode 0600, gitignored) BEFORE this script runs. We MUST
-- NOT create the role here nor embed any password literal — doing so would be
-- both dead code (the role already exists when this runs) and a committed
-- credential leak. The role and its password are therefore env-driven only.

-- Create the database if it doesn't exist
SELECT 'CREATE DATABASE helixcode_prod'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'helixcode_prod')\gexec

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE helixcode_prod TO helix;

-- Connect to the database and create extensions
\c helixcode_prod

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Grant permissions to the helix user
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO helix;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO helix;
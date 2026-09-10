-- Setup script for pgvector example
-- Run this with: psql -h localhost -p 5433 -U test_user -d eino_test -f setup.sql

-- Create pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Create documents table
CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    embedding vector(3),  -- 3 dimensions for the mock embedder
    metadata JSONB
);

-- Create index for vector similarity search (optional but recommended)
CREATE INDEX IF NOT EXISTS documents_embedding_idx ON documents USING hnsw (embedding vector_cosine_ops);

-- Verify setup
\d documents

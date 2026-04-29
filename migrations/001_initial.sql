-- LibraryMS initial schema for PostgreSQL
-- Safe to run multiple times (idempotent)

CREATE TABLE IF NOT EXISTS users (
    id          TEXT        PRIMARY KEY,
    username    TEXT        UNIQUE NOT NULL,
    email       TEXT        UNIQUE NOT NULL,
    password_hash TEXT      NOT NULL,
    role        TEXT        NOT NULL DEFAULT 'user',
    created_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS books (
    id               TEXT        PRIMARY KEY,
    isbn             TEXT,
    title            TEXT        NOT NULL,
    author           TEXT        NOT NULL,
    genre            TEXT,
    description      TEXT,
    published_year   INTEGER,
    total_copies     INTEGER     NOT NULL DEFAULT 1,
    available_copies INTEGER     NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS book_requests (
    id           TEXT        PRIMARY KEY,
    user_id      TEXT        NOT NULL REFERENCES users(id),
    book_id      TEXT        NOT NULL REFERENCES books(id),
    status       TEXT        NOT NULL DEFAULT 'pending',
    notes        TEXT,
    requested_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS checkouts (
    id             TEXT        PRIMARY KEY,
    user_id        TEXT        NOT NULL REFERENCES users(id),
    book_id        TEXT        NOT NULL REFERENCES books(id),
    clerk_id       TEXT        NOT NULL REFERENCES users(id),
    checked_out_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    due_date       TIMESTAMPTZ NOT NULL,
    returned_at    TIMESTAMPTZ
);

-- Indices for common look-up patterns
CREATE INDEX IF NOT EXISTS idx_book_requests_user   ON book_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_book_requests_status ON book_requests(status);
CREATE INDEX IF NOT EXISTS idx_checkouts_user       ON checkouts(user_id);
CREATE INDEX IF NOT EXISTS idx_checkouts_book       ON checkouts(book_id);
CREATE INDEX IF NOT EXISTS idx_checkouts_returned   ON checkouts(returned_at) WHERE returned_at IS NULL;

-- 001_initial_schema.sql
-- Initial schema for transaction service

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS accounts (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_number   VARCHAR(20) UNIQUE NOT NULL,
    account_type     VARCHAR(50) NOT NULL,
    bank_code        VARCHAR(10) NOT NULL,
    balance          NUMERIC(20, 2) NOT NULL DEFAULT 0.00,
    held_balance     NUMERIC(20, 2) NOT NULL DEFAULT 0.00,
    available_balance NUMERIC(20, 2) GENERATED ALWAYS AS (balance - held_balance) STORED,
    currency         VARCHAR(3) NOT NULL DEFAULT 'NGN',
    status           VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    version          BIGINT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TYPE transaction_type AS ENUM (
    'TRANSFER', 'DEPOSIT', 'WITHDRAWAL', 'RETRIEVE', 'UNDO', 'ADJUST', 'HOLD', 'RELEASE'
);

CREATE TYPE transaction_status AS ENUM (
    'PENDING', 'PROCESSING', 'COMPLETED', 'FAILED', 'REVERSED'
);

CREATE TABLE IF NOT EXISTS transactions (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    idempotency_key  VARCHAR(255) UNIQUE NOT NULL,
    type             transaction_type NOT NULL,
    status           transaction_status NOT NULL DEFAULT 'PENDING',
    account_id       UUID NOT NULL REFERENCES accounts(id),
    to_account_id    UUID REFERENCES accounts(id),
    amount           NUMERIC(20, 2) NOT NULL,
    currency         VARCHAR(3) NOT NULL DEFAULT 'NGN',
    description      TEXT,
    route            VARCHAR(20),
    retry_count      INT NOT NULL DEFAULT 0,
    error_message    TEXT,
    metadata         JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS holds (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id     UUID NOT NULL REFERENCES accounts(id),
    transaction_id UUID REFERENCES transactions(id),
    amount         NUMERIC(20, 2) NOT NULL,
    status         VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    expires_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID REFERENCES transactions(id),
    account_id     UUID REFERENCES accounts(id),
    event_type     VARCHAR(100) NOT NULL,
    event_data     JSONB,
    operator_id    VARCHAR(255),
    ip_address     VARCHAR(45),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

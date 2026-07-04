-- Aurelia: Beastbound — core relational schema (PostgreSQL 15+).
--
-- Design notes / engineering decisions:
--  * Postgres is the system of record for anything that must be consistent and
--    auditable: accounts, the creature collection, the inventory ledger, trades
--    and match results. We use it (not a document store) because these have real
--    invariants — you cannot dupe an item or own a creature twice — and Postgres
--    gives us transactions and FK constraints to enforce them for free.
--  * High-churn, low-consistency data (sessions, matchmaking queue, leaderboards,
--    presence, rate limits) lives in Redis, NOT here. See db/redis_keys.md.
--  * Analytics events stream to BigQuery/ClickHouse, NOT here, to keep OLTP lean.
--  * Every mutable row carries updated_at; triggers keep it current for CDC.
--  * UUIDv7 (time-ordered) primary keys give us index locality without exposing
--    sequential ids to clients.

CREATE EXTENSION IF NOT EXISTS "pgcrypto"; -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS "citext";   -- case-insensitive email column

-- ---------------------------------------------------------------------------
-- Accounts & auth
-- ---------------------------------------------------------------------------
CREATE TABLE accounts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           CITEXT UNIQUE NOT NULL,
    -- Password hash is argon2id; NULL when the account is OAuth-only.
    password_hash   TEXT,
    display_name    TEXT NOT NULL,
    region          TEXT NOT NULL DEFAULT 'auto',      -- matchmaking region hint
    status          TEXT NOT NULL DEFAULT 'active',     -- active|banned|shadow
    mmr             INTEGER NOT NULL DEFAULT 1000,       -- ranked rating (Glicko-2 r)
    mmr_rd          REAL NOT NULL DEFAULT 350,           -- rating deviation
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at   TIMESTAMPTZ
);
CREATE INDEX idx_accounts_mmr ON accounts (mmr) WHERE status = 'active';

-- Refresh tokens are stored hashed so a DB leak cannot mint sessions.
CREATE TABLE auth_tokens (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id    UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    token_hash    TEXT NOT NULL,
    device_id     TEXT,
    expires_at    TIMESTAMPTZ NOT NULL,
    revoked       BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_auth_tokens_account ON auth_tokens (account_id) WHERE NOT revoked;

-- ---------------------------------------------------------------------------
-- Creature collection
-- ---------------------------------------------------------------------------
-- Species are static design data loaded from the JSON seed, NOT a hot table; we
-- mirror a thin copy here only for FK integrity and analytics joins. The game
-- server reads species from the in-memory Dex, not from this table.
CREATE TABLE species (
    dex_id     INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    rarity     SMALLINT NOT NULL,
    element1   SMALLINT NOT NULL,
    element2   SMALLINT NOT NULL
);

-- A creature_instance is a single caught creature owned by exactly one account.
-- This is the table most sensitive to dupe exploits, hence strict ownership FK
-- and an append-only acquisition source for audit.
CREATE TABLE creature_instances (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id      UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    dex_id        INTEGER NOT NULL REFERENCES species(dex_id),
    nickname      TEXT,
    level         SMALLINT NOT NULL DEFAULT 1 CHECK (level BETWEEN 1 AND 100),
    xp            INTEGER NOT NULL DEFAULT 0,
    -- IVs/EVs/nature/ability packed into JSONB: read together, never queried
    -- field-by-field, so a document column is the right call here.
    genes         JSONB NOT NULL,    -- {ivs, evs, nature, ability}
    skill_ids     TEXT[] NOT NULL,
    is_shiny      BOOLEAN NOT NULL DEFAULT false,
    acquired_via  TEXT NOT NULL,     -- wild|trade|reward|event
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_creatures_owner ON creature_instances (owner_id);
CREATE INDEX idx_creatures_dex ON creature_instances (dex_id);

-- The player's active battle team (max 6, ordered).
CREATE TABLE teams (
    account_id   UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    slot         SMALLINT NOT NULL CHECK (slot BETWEEN 0 AND 5),
    creature_id  UUID NOT NULL REFERENCES creature_instances(id) ON DELETE CASCADE,
    PRIMARY KEY (account_id, slot)
);

-- ---------------------------------------------------------------------------
-- Inventory (items, currency) — ledgered for anti-dupe auditability
-- ---------------------------------------------------------------------------
CREATE TABLE inventory (
    account_id  UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    item_id     TEXT NOT NULL,
    quantity    INTEGER NOT NULL CHECK (quantity >= 0),
    PRIMARY KEY (account_id, item_id)
);

-- Every balance change is an immutable ledger row. Current balances above are a
-- materialized convenience; the ledger is the truth and is reconciled nightly.
CREATE TABLE currency_ledger (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id  UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    currency    TEXT NOT NULL,        -- soft_coin | aurelium (premium)
    delta       BIGINT NOT NULL,
    reason      TEXT NOT NULL,        -- purchase|reward|battle|trade|refund
    ref_id      TEXT,                 -- idempotency / receipt id
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (currency, ref_id)         -- replay-proof: a receipt applies once
);
CREATE INDEX idx_ledger_account ON currency_ledger (account_id, currency);

-- ---------------------------------------------------------------------------
-- Trades (two-phase, escrowed, audited — the classic dupe attack surface)
-- ---------------------------------------------------------------------------
CREATE TABLE trades (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    initiator_id  UUID NOT NULL REFERENCES accounts(id),
    partner_id    UUID NOT NULL REFERENCES accounts(id),
    state         TEXT NOT NULL DEFAULT 'open', -- open|locked|committed|cancelled
    initiator_ok  BOOLEAN NOT NULL DEFAULT false,
    partner_ok    BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    committed_at  TIMESTAMPTZ
);
CREATE TABLE trade_items (
    trade_id     UUID NOT NULL REFERENCES trades(id) ON DELETE CASCADE,
    from_account UUID NOT NULL REFERENCES accounts(id),
    creature_id  UUID REFERENCES creature_instances(id),
    item_id      TEXT,
    quantity     INTEGER,
    PRIMARY KEY (trade_id, from_account, creature_id, item_id)
);

-- ---------------------------------------------------------------------------
-- Match history (authoritative result + replay seed for anti-cheat re-sim)
-- ---------------------------------------------------------------------------
CREATE TABLE matches (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mode          TEXT NOT NULL,        -- ranked|casual|gym|raid
    seed          BIGINT NOT NULL,      -- replay/anti-cheat re-simulation seed
    player_a      UUID REFERENCES accounts(id),
    player_b      UUID REFERENCES accounts(id),
    winner        SMALLINT,             -- 0|1|-1(draw)|NULL(in progress)
    turns         INTEGER,
    final_digest  BIGINT,               -- engine state hash at conclusion
    started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at      TIMESTAMPTZ
);
CREATE INDEX idx_matches_player_a ON matches (player_a, started_at DESC);
CREATE INDEX idx_matches_player_b ON matches (player_b, started_at DESC);

-- Per-turn action log enables full replay and dispute resolution. Partitioned by
-- month in production (DDL omitted here) to keep the hot set small.
CREATE TABLE match_actions (
    match_id   UUID NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    turn       INTEGER NOT NULL,
    side       SMALLINT NOT NULL,
    action     JSONB NOT NULL,
    digest     BIGINT NOT NULL,
    PRIMARY KEY (match_id, turn, side)
);

-- updated_at trigger ------------------------------------------------------
CREATE OR REPLACE FUNCTION touch_updated_at() RETURNS trigger AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END; $$ LANGUAGE plpgsql;

CREATE TRIGGER trg_accounts_touch BEFORE UPDATE ON accounts
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER trg_creatures_touch BEFORE UPDATE ON creature_instances
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

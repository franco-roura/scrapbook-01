-- +goose Up
-- Prepaid credits. Amounts are micro-dollars of model cost (1 = $0.000001):
-- a pack sold for $5 buys $4 of model cost (80% to models, 20% margin), so
-- it adds 4_000_000. Every AI call subtracts what it actually cost.

CREATE TABLE credit_balances (
    user_id    uuid PRIMARY KEY,
    balance    bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE credit_ledger (
    id         bigserial PRIMARY KEY,
    user_id    uuid NOT NULL,
    -- Positive for grants and purchases, negative for usage.
    amount     bigint NOT NULL,
    kind       text NOT NULL CHECK (kind IN ('starter', 'grant', 'purchase', 'usage')),
    -- For usage: the step ("classify", "search"), the item, and per-model
    -- token counts. For purchases: the pack and the payment reference.
    item_id    uuid,
    detail     jsonb NOT NULL DEFAULT '{}',
    -- Payment references are unique so a webhook delivered twice can't
    -- credit twice.
    reference  text UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX credit_ledger_user ON credit_ledger (user_id, created_at DESC);

-- +goose Down
DROP TABLE credit_ledger;
DROP TABLE credit_balances;

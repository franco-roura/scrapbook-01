-- name: EnsureStarterCredits :one
-- Gives a new user their free starter credits once, and returns the
-- balance either way.
WITH created AS (
    INSERT INTO credit_balances (user_id, balance)
    VALUES (@user_id, @starter::bigint)
    ON CONFLICT (user_id) DO NOTHING
    RETURNING user_id, balance
), logged AS (
    INSERT INTO credit_ledger (user_id, amount, kind)
    SELECT user_id, balance, 'starter' FROM created WHERE balance <> 0
)
SELECT balance FROM created
UNION ALL
SELECT balance FROM credit_balances WHERE user_id = @user_id AND NOT EXISTS (SELECT 1 FROM created);

-- name: AddCredits :one
-- Records a ledger entry and moves the balance in one statement. A repeated
-- reference (a payment webhook delivered twice) changes nothing and
-- returns no row.
WITH entry AS (
    INSERT INTO credit_ledger (user_id, amount, kind, item_id, detail, reference)
    VALUES (@user_id, @amount, @kind, sqlc.narg(item_id), @detail, sqlc.narg(reference))
    ON CONFLICT (reference) DO NOTHING
    RETURNING user_id, amount
)
INSERT INTO credit_balances (user_id, balance)
SELECT user_id, amount FROM entry
ON CONFLICT (user_id) DO UPDATE
SET balance = credit_balances.balance + EXCLUDED.balance, updated_at = now()
RETURNING balance;

-- name: GetCreditBalance :one
SELECT balance FROM credit_balances WHERE user_id = @user_id;

-- name: ListCreditLedger :many
SELECT id, amount, kind, item_id, detail, created_at
FROM credit_ledger
WHERE user_id = @user_id
ORDER BY created_at DESC, id DESC
LIMIT @lim;

-- name: AverageClassifyCost :one
-- Typical cost of sorting one item for this user (last 50), to estimate
-- how many saves the balance covers.
SELECT COALESCE(avg(-amount), 0)::float8
FROM (
    SELECT amount FROM credit_ledger
    WHERE user_id = @user_id AND kind = 'usage' AND detail->>'step' = 'classify'
    ORDER BY created_at DESC
    LIMIT 50
) recent;

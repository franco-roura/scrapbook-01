-- name: UpsertDevice :exec
-- A token moves to whoever signed in on that phone last.
INSERT INTO devices (token, user_id, platform)
VALUES (@token, @user_id, @platform)
ON CONFLICT (token) DO UPDATE
SET user_id = EXCLUDED.user_id, platform = EXCLUDED.platform, updated_at = now();

-- name: DeleteDevice :exec
DELETE FROM devices WHERE token = @token AND user_id = @user_id;

-- name: DeleteDeviceTokens :exec
-- Tokens Expo reports as no longer registered.
DELETE FROM devices WHERE token = ANY(@tokens::text[]);

-- name: ListDeviceTokens :many
SELECT token FROM devices WHERE user_id = @user_id;

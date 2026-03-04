-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at)
VALUES (
    $1,
    NOW(),
    NOW(),
    $2,
    NOW() + INTERVAL '60 DAY'
)
RETURNING *;

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens
WHERE token = $1;

-- name: GetUserFromRefreshToken :one
SELECT * FROM users
INNER JOIN refresh_tokens
ON users.id = refresh_tokens.user_id
WHERE refresh_tokens.token = $1;

-- name: SetRevokedAtForRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = NOW(), updated_at = NOW()
WHERE token = $1;
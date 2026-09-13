-- name: GetUserById :one
SELECT * FROM users
WHERE id = ?
LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = ?
LIMIT 1;

-- name: GetAllUsers :many
SELECT id, username, created_at FROM users;

-- name: CreateUser :one
INSERT INTO users (username)
VALUES (?)
RETURNING id, username, created_at;

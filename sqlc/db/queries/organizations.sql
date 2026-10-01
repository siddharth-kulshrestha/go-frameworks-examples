-- name: CreateOrganization :one
INSERT INTO organizations (id, name, slug)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetOrganization :one
SELECT *
FROM organizations
WHERE id = $1;

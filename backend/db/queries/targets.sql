-- name: ListTargets :many
SELECT id, name, primary_domain, platform, program_url, description, program_rules,
       program_restrictions, rate_limit_per_minute, status, created_at, updated_at, archived_at
FROM targets WHERE status = ? ORDER BY updated_at DESC LIMIT ?;

-- name: GetTarget :one
SELECT id, name, primary_domain, platform, program_url, description, program_rules,
       program_restrictions, rate_limit_per_minute, status, created_at, updated_at, archived_at
FROM targets WHERE id = ?;

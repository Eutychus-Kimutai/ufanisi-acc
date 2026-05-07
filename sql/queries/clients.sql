-- name: GetClientByID :one
SELECT id, name, client_type FROM clients WHERE id = $1;

-- name: GetInvestorCapitalAccount :one
SELECT id FROM accounts WHERE name = 'Investor Capital Account' LIMIT 1;

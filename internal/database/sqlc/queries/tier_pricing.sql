-- name: GetTierPricingByTierId :many
SELECT 
    tier_base_pricing.*, api_endpoint.endpoint_name,
    COUNT(tier_base_pricing_id) OVER() AS total_items
FROM tier_base_pricing
INNER JOIN api_endpoint ON tier_base_pricing.api_endpoint_id = api_endpoint.api_endpoint_id
WHERE subscription_tier_id = $1
LIMIT $2 OFFSET $3;

-- name: CreateTierPricing :one 
INSERT INTO tier_base_pricing (subscription_tier_id, api_endpoint_id, base_cost_per_call, base_rate_limit) 
VALUES ($1, $2, $3, $4)
RETURNING tier_base_pricing_id;

-- name: CreateTierPricings :copyfrom
INSERT INTO tier_base_pricing (subscription_tier_id, api_endpoint_id, base_cost_per_call, base_rate_limit) 
VALUES ($1, $2, $3, $4);

-- name: UpdateTierPricingByTierId :execresult
UPDATE tier_base_pricing
SET 
    base_cost_per_call = $1,
    base_rate_limit = $2,
    api_endpoint_id = $3
WHERE subscription_tier_id = $4;

-- name: UpdateTierPricingById :execresult
UPDATE tier_base_pricing
SET 
    base_cost_per_call = $1,
    base_rate_limit = $2,
    api_endpoint_id = $3
WHERE tier_base_pricing_id = $4;

-- name: DeleteTierPricingByTierId :exec
DELETE FROM tier_base_pricing
WHERE subscription_tier_id = $1;

-- name: DeleteTierPricingById :exec
DELETE FROM tier_base_pricing
WHERE tier_base_pricing_id = $1;
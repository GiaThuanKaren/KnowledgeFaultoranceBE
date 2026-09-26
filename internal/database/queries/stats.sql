-- name: IncrementDailyStat :one
INSERT INTO daily_stats (user_id, stat_date, activity_count)
VALUES ($1, $2, 1)
ON CONFLICT (user_id, stat_date)
DO UPDATE SET activity_count = daily_stats.activity_count + 1
RETURNING *;

-- name: GetDailyStatsByYear :many
SELECT * FROM daily_stats
WHERE user_id = $1 AND EXTRACT(YEAR FROM stat_date)::int = sqlc.arg('year')::int
ORDER BY stat_date ASC;

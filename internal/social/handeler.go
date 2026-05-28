package social

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type SocialHandeler struct {
	dbPool *pgxpool.Pool
	redis  *redis.Client
}

func NewSocialHandeler(pool *pgxpool.Pool, redisC *redis.Client) (*SocialHandeler, error) {
	h := &SocialHandeler{
		dbPool: pool,
		redis:  redisC,
	}
	return h, nil
}

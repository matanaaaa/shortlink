package repo

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

const nullValue = "__NULL__"

type RedisRepo struct {
	RDB *redis.Client
}

func key(code string) string { return "sl:code:" + code }

func (r *RedisRepo) Get(ctx context.Context, code string) (val string, hit bool, isNull bool, err error) {
	v, err := r.RDB.Get(ctx, key(code)).Result()
	if err == redis.Nil {
		return "", false, false, nil
	}
	if err != nil {
		return "", false, false, err
	}
	if v == nullValue {
		return "", true, true, nil
	}
	return v, true, false, nil
}

func (r *RedisRepo) SetURL(ctx context.Context, code, longURL string, ttl time.Duration) error {
	return r.RDB.Set(ctx, key(code), longURL, ttl).Err()
}

func (r *RedisRepo) SetNull(ctx context.Context, code string, ttl time.Duration) error {
	return r.RDB.Set(ctx, key(code), nullValue, ttl).Err()
}

func (r *RedisRepo) Del(ctx context.Context, code string) error {
	return r.RDB.Del(ctx, key(code)).Err()
}

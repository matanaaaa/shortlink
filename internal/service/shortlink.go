package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"strings"
	"time"

	"shortlink/internal/repo"
)

type Service struct {
	My   *repo.MySQLRepo
	Rds  *repo.RedisRepo
	Base string
}

var ErrNotFound = errors.New("not found")

func isDupKey(err error) bool {
	// MySQL duplicate key error string contains "Duplicate entry"
	// 简化处理：面试里可以说生产建议用 mysql 错误码 1062 来判断
	return err != nil && strings.Contains(err.Error(), "Duplicate entry")
}

// 生成随机 base62 code（8位），冲突则重试
func (s *Service) Shorten(ctx context.Context, longURL string) (code string, shortURL string, err error) {
	longURL = strings.TrimSpace(longURL)
	if longURL == "" || len(longURL) > 4000 {
		return "", "", errors.New("invalid long_url")
	}

	for i := 0; i < 5; i++ {
		c, err := randomBase62(8)
		if err != nil {
			return "", "", err
		}
		err = s.My.Insert(ctx, c, longURL)
		if err == nil {
			// 写库成功后删缓存（闭环一致性：写后删）
			_ = s.Rds.Del(ctx, c)
			return c, s.Base + "/r/" + c, nil
		}
		if isDupKey(err) {
			continue
		}
		return "", "", err
	}
	return "", "", errors.New("too many collisions")
}

func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 16 {
		return "", ErrNotFound
	}

	// P1：Cache Aside + 空值缓存（穿透）
	if s.Rds != nil {
		v, hit, isNull, err := s.Rds.Get(ctx, code)
		if err == nil && hit {
			if isNull {
				return "", ErrNotFound
			}
			return v, nil
		}
	}

	longURL, ok, err := s.My.GetLongURL(ctx, code)
	if err != nil {
		return "", err
	}
	if !ok {
		if s.Rds != nil {
			_ = s.Rds.SetNull(ctx, code, 30*time.Second)
		}
		return "", ErrNotFound
	}

	if s.Rds != nil {
		_ = s.Rds.SetURL(ctx, code, longURL, 24*time.Hour)
	}
	return longURL, nil
}

// --- utils ---
const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func randomBase62(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = base62[int(b[i])%len(base62)]
	}
	return string(out), nil
}

// 可选：用于 main 里 ping db
func PingDB(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

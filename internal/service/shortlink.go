package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"strings"
	"time"
	"log"

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

	// 1) First check cache
	if s.Rds != nil {
		v, hit, isNull, err := s.Rds.Get(ctx, code)
		if err == nil && hit {
			if isNull {
				return "", ErrNotFound
			}
			return v, nil
		}
	}

	// 2) Cache miss -> mutex lock to prevent breakdown
	if s.Rds != nil {
		token, _ := randomBase62(12) // reuse your random generator
		locked, err := s.Rds.TryLock(ctx, code, token, 3*time.Second)
		if err == nil && locked {
			// We are the "single flight" winner
			defer func() { _ = s.Rds.Unlock(ctx, code, token) }()

			// 2.1 Double check cache again (maybe another request has filled it)
			v, hit, isNull, err := s.Rds.Get(ctx, code)
			if err == nil && hit {
				if isNull {
					return "", ErrNotFound
				}
				return v, nil
			}

			log.Println("lock winner, hitting DB for code=", code)

			// 2.2 Query DB and fill cache
			longURL, ok, err := s.My.GetLongURL(ctx, code)
			if err != nil {
				return "", err
			}
			if !ok {
				_ = s.Rds.SetNull(ctx, code, 30*time.Second)
				return "", ErrNotFound
			}
			_ = s.Rds.SetURL(ctx, code, longURL, 24*time.Hour)
			return longURL, nil
		}

		// 3) Not locked -> wait a bit and retry cache (do NOT hit DB)
		for i := 0; i < 5; i++ {
			time.Sleep(30 * time.Millisecond)

			v, hit, isNull, err := s.Rds.Get(ctx, code)
			if err == nil && hit {
				if isNull {
					return "", ErrNotFound
				}
				return v, nil
			}
		}
		// If still not in cache after retries, fall through to DB as last resort
	}

	// 4) Last resort: query DB
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

func PingDB(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

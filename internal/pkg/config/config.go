package config

import "os"

type Config struct {
	MySQLDSN  string
	RedisAddr string
	BaseURL   string
	Port      string
}

func Get() Config {
	c := Config{
		MySQLDSN:  os.Getenv("MYSQL_DSN"),
		RedisAddr: os.Getenv("REDIS_ADDR"),
		BaseURL:   os.Getenv("BASE_URL"),
		Port:      os.Getenv("PORT"),
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	return c
}

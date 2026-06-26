package config

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Environment string
	HTTPAddr    string
	DatabaseURL string
}

func Load() (Config, error) {
	loadDotEnvFiles(".env", "../.env")

	cfg := Config{
		Environment: env("APP_ENV", "development"),
		HTTPAddr:    env("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = postgresURLFromEnv()
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL or POSTGRES_* variables are required")
	}

	return cfg, nil
}

func loadDotEnvFiles(paths ...string) {
	for _, path := range paths {
		_ = loadDotEnv(path)
	}
}

func postgresURLFromEnv() string {
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbName := os.Getenv("POSTGRES_DB")
	port := env("POSTGRES_PORT", "5432")
	host := env("POSTGRES_HOST", "localhost")

	if user == "" || password == "" || dbName == "" {
		return ""
	}

	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
		Path:   dbName,
	}

	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()

	return u.String()
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid env line: %s", line)
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			return fmt.Errorf("invalid env key in line: %s", line)
		}

		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

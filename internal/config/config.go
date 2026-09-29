package config

import (
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Host, Port, DatabasePath, SessionSecret, PublicURL string
	CookieSecure                                       bool
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("read .env: %w", err)
	}
	c := Config{Host: os.Getenv("HOST"), Port: os.Getenv("PORT"), DatabasePath: os.Getenv("DATABASE_PATH"), SessionSecret: os.Getenv("SESSION_SECRET"), PublicURL: strings.TrimRight(os.Getenv("PUBLIC_URL"), "/")}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return c, errors.New("PORT must be between 1 and 65535; copy .env.example to .env")
	}
	if c.DatabasePath == "" {
		return c, errors.New("DATABASE_PATH is required")
	}
	if len(c.SessionSecret) < 32 {
		return c, errors.New("SESSION_SECRET must have at least 32 characters; generate a random secret")
	}
	c.CookieSecure, err = strconv.ParseBool(os.Getenv("COOKIE_SECURE"))
	if err != nil {
		return c, errors.New("COOKIE_SECURE must be true or false")
	}
	u, err := url.Parse(c.PublicURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return c, errors.New("PUBLIC_URL must be an http(s) origin without a path")
	}
	if u.Scheme == "https" && !c.CookieSecure {
		return c, errors.New("COOKIE_SECURE must be true for HTTPS")
	}
	if u.Scheme == "http" && c.CookieSecure {
		return c, errors.New("COOKIE_SECURE requires an HTTPS PUBLIC_URL")
	}
	return c, nil
}

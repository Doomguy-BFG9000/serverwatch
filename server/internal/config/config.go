package config

import "os"

type Config struct {
	HTTPAddr string
}

func Load() Config {
	addr := os.Getenv("SERVERWATCH_HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	return Config{
		HTTPAddr: addr,
	}
}

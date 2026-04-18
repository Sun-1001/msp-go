package redis

import (
	"testing"
	"time"

	"mathstudy/backend-go/internal/platform/config"
)

func TestNewClientUsesConfig(t *testing.T) {
	cfg := config.Config{
		RedisHost:           "cache",
		RedisPort:           6380,
		RedisPassword:       "secret",
		RedisDB:             2,
		RedisMaxConnections: 9,
		RedisSocketTimeout:  1500 * time.Millisecond,
		RedisConnectTimeout: 2 * time.Second,
	}

	client := NewClient(cfg)
	defer client.Close()

	options := client.Options()
	if options.Addr != "cache:6380" {
		t.Fatalf("Addr = %q", options.Addr)
	}
	if options.Password != "secret" {
		t.Fatalf("Password = %q", options.Password)
	}
	if options.DB != 2 {
		t.Fatalf("DB = %d", options.DB)
	}
	if options.PoolSize != 9 {
		t.Fatalf("PoolSize = %d", options.PoolSize)
	}
}

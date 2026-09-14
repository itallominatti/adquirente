package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Lock struct {
	client *goredis.Client
	tokens map[string]string
}

func NewLock(addr string) *Lock {
	return &Lock{client: goredis.NewClient(&goredis.Options{Addr: addr}), tokens: map[string]string{}}
}

func (l *Lock) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return false, err
	}
	token := hex.EncodeToString(buf)
	ok, err := l.client.SetNX(ctx, key, token, ttl).Result() // NX: só se não existir. Atômico.
	if err != nil {
		return false, err
	}
	if ok {
		l.tokens[key] = token
	}
	return ok, nil
}

var releaseScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0`)

func (l *Lock) Release(ctx context.Context, key string) error {
	token, ok := l.tokens[key]
	if !ok {
		return nil
	}
	delete(l.tokens, key)
	return releaseScript.Run(ctx, l.client, []string{key}, token).Err()
}

func (l *Lock) Ping(ctx context.Context) error { return l.client.Ping(ctx).Err() }
func (l *Lock) Close() error                   { return l.client.Close() }

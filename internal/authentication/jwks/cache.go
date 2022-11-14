package jwks

import (
	"fmt"
	"time"

	"github.com/envoyproxy/gateway/internal/cache"
)

func newCache(ttl time.Duration) *cache.Cache {
	if ttl < -1 {
		panic(fmt.Sprintf("invalid ttl: %d", ttl))
	}
	return cache.New(pubKeyExpiration, cacheCleanupInternal)
}

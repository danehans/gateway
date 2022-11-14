package jwks

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/semaphore"
	"gopkg.in/go-jose/go-jose.v2"

	"github.com/envoyproxy/gateway/internal/cache"
)

const (
	// pubKeyExpiration is the lifetime of a cached JWT public key.
	pubKeyExpiration = 12 * time.Hour
	// cacheCleanupInternal is the interval for the cache manager to remove
	// expired JWT public key.
	cacheCleanupInternal = 24 * time.Hour
)

type client struct {
	source  Source
	cache   *cache.Cache
	refresh time.Duration
	sem     *semaphore.Weighted
}

type cacheEntry struct {
	jwk     *jose.JSONWebKey
	refresh int64
}

// newClient creates a new JWKS client based on the provided input and default cache settings.
func newClient(src Source, refresh time.Duration, ttl time.Duration) *client {
	if refresh >= ttl {
		panic(fmt.Sprintf("invalid refresh %v, must be less than or equal to ttl: %v", refresh, ttl))
	}
	if refresh < 0 {
		panic(fmt.Sprintf("invalid refresh: %v", refresh))
	}
	return &client{
		source:  src,
		cache:   newCache(ttl),
		refresh: refresh,
		sem:     semaphore.NewWeighted(1),
	}
}

func (c *client) GetKey(ctx context.Context, keyId string, use string) (jwk *jose.JSONWebKey, err error) {
	logger, err := logr.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	val, found := c.cache.Get(keyId)
	if found {
		entry := val.(*cacheEntry)
		if time.Now().After(time.Unix(entry.refresh, 0)) && c.sem.TryAcquire(1) {
			go func() {
				defer c.sem.Release(1)
				if _, err := c.refreshKey(ctx, keyId, use); err != nil {
					logger.Error(err, "unable to refresh key")
				}
			}()
		}
		return entry.jwk, nil
	} else {
		return c.refreshKey(ctx, keyId, use)
	}
}

func (c *client) refreshKey(ctx context.Context, keyId string, use string) (*jose.JSONWebKey, error) {
	jwk, err := c.getKey(ctx, keyId, use)
	if err != nil {
		return nil, err
	}

	c.store(keyId, jwk)
	return jwk, nil
}

func (c *client) store(keyId string, jwk *jose.JSONWebKey) {
	ce := &cacheEntry{
		jwk:     jwk,
		refresh: time.Now().Add(c.refresh).Unix(),
	}
	c.cache.Store(keyId, ce, pubKeyExpiration)
}

func (c *client) getKey(ctx context.Context, keyId string, use string) (*jose.JSONWebKey, error) {
	jsonWebKeySet, err := c.source.GetJWKS(ctx)
	if err != nil {
		return nil, err
	}

	keys := jsonWebKeySet.Key(keyId)
	if len(keys) == 0 {
		return nil, fmt.Errorf("JWK key %s not found", keyId)
	}

	for _, jwk := range keys {
		return &jwk, nil
	}
	return nil, fmt.Errorf("JWK is not found %s, use: %s", keyId, use)
}

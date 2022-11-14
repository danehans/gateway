package jwks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"gopkg.in/go-jose/go-jose.v2"
)

type Source interface {
	GetJWKS(ctx context.Context) (*jose.JSONWebKeySet, error)
}

type RemoteSource struct {
	client  *http.Client
	jwksUri string
}

func NewRemoteSource(jwksUri string) *RemoteSource {
	return &RemoteSource{
		client:  new(http.Client),
		jwksUri: jwksUri,
	}
}

func (s *RemoteSource) GetJWKS(ctx context.Context) (*jose.JSONWebKeySet, error) {
	logger, err := logr.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	logger.Info("Fetching JWKS", "uri", s.jwksUri)

	req, err := http.NewRequest("GET", s.jwksUri, nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request failed, status: %d", resp.StatusCode)
	}

	jwks := new(jose.JSONWebKeySet)
	if err = json.NewDecoder(resp.Body).Decode(jwks); err != nil {
		return nil, err
	}

	return jwks, err
}

/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 */

package qiniu

import (
	"os"

	"github.com/goplus/xai/spec/minimax_h3"
)

// Service wraps the MiniMax-H3 xai service with a Qiniu client.
type Service struct {
	*minimax_h3.Service
	client  *Client
	backend *backend
}

// SetApiKey updates the API key used by subsequent requests.
func (s *Service) SetApiKey(apiKey string) { s.client.SetApiKey(apiKey) }

// MiniMaxH3Service returns the embedded spec service for Operation.Call.
func (s *Service) MiniMaxH3Service() *minimax_h3.Service { return s.Service }

// NewService creates a Qiniu-backed MiniMax-H3 service.
func NewService(apiKey string, opts ...ClientOption) *Service {
	if apiKey == "" {
		apiKey = os.Getenv("QINIU_API_KEY")
	}
	client := NewClient(apiKey, opts...)
	backend := newBackend(client)
	return &Service{
		Service: minimax_h3.NewWithBackend(backend),
		client:  client,
		backend: backend,
	}
}

// Register registers a Qiniu-backed minimax_h3:// service globally.
func Register(apiKey string, opts ...ClientOption) {
	minimax_h3.Register(NewService(apiKey, opts...))
}

/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 */

package minimax_h3

import (
	"context"

	xai "github.com/goplus/xai/spec"
)

// Backend is the transport boundary for MiniMax-H3 asynchronous tasks.
type Backend interface {
	Submit(ctx context.Context, model xai.Model, params xai.Params) (xai.OperationResponse, error)
	GetTaskStatus(ctx context.Context, requestID string) (xai.OperationResponse, error)
}

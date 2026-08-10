/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 */

// Package minimax_h3 defines the xai operation contract for MiniMax-H3 video
// generation through the FAL Queue API.
package minimax_h3

import (
	"context"
	"strings"
)

import xai "github.com/goplus/xai/spec"

// Scheme is the URI scheme for MiniMax-H3.
const Scheme = "minimax_h3"

// ModelMinimaxH3 is the Qiniu MaaS model ID.
const ModelMinimaxH3 = "minimax/minimax-h3"

var defaultVideoModels = []string{ModelMinimaxH3}

// IsVideoModel reports whether model is supported by this spec.
func IsVideoModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), ModelMinimaxH3)
}

// VideoModels returns the built-in MiniMax-H3 model IDs.
func VideoModels() []string {
	return append([]string(nil), defaultVideoModels...)
}

// Register registers a MiniMax-H3 service with xai.
func Register(svc xai.Service) {
	xai.Register(Scheme, func(ctx context.Context, uri string) (xai.Service, error) {
		return svc, nil
	})
}

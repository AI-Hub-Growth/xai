/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 */

package minimax_h3

import (
	"context"
	"time"

	xai "github.com/goplus/xai/spec"
	"github.com/goplus/xai/spec/vidu/video"
)

// NewOutputVideos creates standard xai video results.
func NewOutputVideos(urls []string) xai.Results { return video.NewOutputVideos(urls) }

// SyncOperationResponse is a completed operation response.
type SyncOperationResponse struct{ R xai.Results }

func (p *SyncOperationResponse) Done() bool { return true }
func (p *SyncOperationResponse) Sleep()     {}
func (p *SyncOperationResponse) Retry(context.Context, xai.Service) (xai.OperationResponse, error) {
	return p, nil
}
func (p *SyncOperationResponse) Results() xai.Results { return p.R }
func (p *SyncOperationResponse) TaskID() string       { return "" }

// AsyncOperationResponse represents a queued or running H3 task.
type AsyncOperationResponse struct {
	RetryFunc func(context.Context) (xai.OperationResponse, error)
	SleepDur  time.Duration
	taskID    string
}

// NewAsyncOperationResponse creates a polling response with a two-second default interval.
func NewAsyncOperationResponse(retryFunc func(context.Context) (xai.OperationResponse, error), taskID string) *AsyncOperationResponse {
	return &AsyncOperationResponse{RetryFunc: retryFunc, SleepDur: 2 * time.Second, taskID: taskID}
}

func (p *AsyncOperationResponse) Done() bool { return false }
func (p *AsyncOperationResponse) Sleep() {
	if p.SleepDur > 0 {
		time.Sleep(p.SleepDur)
	}
}
func (p *AsyncOperationResponse) Retry(ctx context.Context, _ xai.Service) (xai.OperationResponse, error) {
	if p.RetryFunc == nil {
		return p, nil
	}
	return p.RetryFunc(ctx)
}
func (p *AsyncOperationResponse) Results() xai.Results { return nil }
func (p *AsyncOperationResponse) TaskID() string       { return p.taskID }

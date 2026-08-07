/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 */

package qiniu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	xai "github.com/goplus/xai/spec"
	"github.com/goplus/xai/spec/minimax_h3"
)

const (
	EndpointTextToVideo      = "/queue/minimax/h3/text-to-video"
	EndpointImageToVideo     = "/queue/minimax/h3/image-to-video"
	EndpointReferenceToVideo = "/queue/minimax/h3/reference-to-video"
	EndpointStatusPrefix     = "/queue/minimax/h3/requests/"
)

const (
	StatusInQueue    = "IN_QUEUE"
	StatusInProgress = "IN_PROGRESS"
	StatusCompleted  = "COMPLETED"
	StatusFailed     = "FAILED"
	StatusCancelled  = "CANCELLED"
	StatusCanceled   = "CANCELED"
)

// ErrTaskFailed marks a terminal upstream failure.
var ErrTaskFailed = errors.New("qiniu-minimax-h3: task failed")

type backend struct{ client *Client }

func newBackend(client *Client) *backend { return &backend{client: client} }

// NewBackend exposes a backend for custom service wiring and tests.
func NewBackend(client *Client) minimax_h3.Backend { return newBackend(client) }

// VideoRequest is the endpoint and JSON body selected for one H3 operation.
type VideoRequest struct {
	Endpoint string
	Body     map[string]any
}

// BuildVideoRequest maps xai params to one of the three MiniMax-H3 endpoints.
func BuildVideoRequest(model string, params *minimax_h3.Params) (*VideoRequest, error) {
	if !minimax_h3.IsVideoModel(model) {
		return nil, fmt.Errorf("qiniu-minimax-h3: unsupported model %q", model)
	}
	if err := minimax_h3.ValidateParams(params); err != nil {
		return nil, err
	}
	body := map[string]any{
		"prompt": params.GetString(minimax_h3.ParamPrompt),
	}
	putInt(body, "duration", params.GetInt(minimax_h3.ParamDuration))
	putString(body, "resolution", params.GetString(minimax_h3.ParamResolution))
	mode := minimax_h3.VideoMode(params)
	switch mode {
	case minimax_h3.VideoModeTextToVideo:
		putString(body, "aspect_ratio", params.GetString(minimax_h3.ParamAspectRatio))
		return &VideoRequest{Endpoint: EndpointTextToVideo, Body: body}, nil
	case minimax_h3.VideoModeImageToVideo, minimax_h3.VideoModeStartEndToVideo:
		putString(body, "image_url", params.GetString(minimax_h3.ParamImageURL))
		if mode == minimax_h3.VideoModeStartEndToVideo {
			putString(body, "end_image_url", params.GetString(minimax_h3.ParamEndImageURL))
		}
		return &VideoRequest{Endpoint: EndpointImageToVideo, Body: body}, nil
	case minimax_h3.VideoModeMultiRefToVideo:
		putStrings(body, "reference_image_urls", params.GetStringSlice(minimax_h3.ParamReferenceImageURLs))
		putStrings(body, "reference_video_urls", params.GetStringSlice(minimax_h3.ParamReferenceVideoURLs))
		putStrings(body, "reference_audio_urls", params.GetStringSlice(minimax_h3.ParamReferenceAudioURLs))
		putString(body, "aspect_ratio", params.GetString(minimax_h3.ParamAspectRatio))
		return &VideoRequest{Endpoint: EndpointReferenceToVideo, Body: body}, nil
	default:
		return nil, fmt.Errorf("qiniu-minimax-h3: unsupported video mode %q", mode)
	}
}

func putString(body map[string]any, name, value string) {
	if strings.TrimSpace(value) != "" {
		body[name] = strings.TrimSpace(value)
	}
}

func putInt(body map[string]any, name string, value *int) {
	if value != nil {
		body[name] = *value
	}
}

func putStrings(body map[string]any, name string, values []string) {
	if len(values) > 0 {
		body[name] = values
	}
}

func (b *backend) Submit(ctx context.Context, model xai.Model, params xai.Params) (xai.OperationResponse, error) {
	p, ok := params.(*minimax_h3.Params)
	if !ok {
		return nil, fmt.Errorf("qiniu-minimax-h3: expected *minimax_h3.Params, got %T", params)
	}
	req, err := BuildVideoRequest(string(model), p)
	if err != nil {
		return nil, err
	}
	raw, err := b.client.PostJSON(ctx, req.Endpoint, req.Body)
	if err != nil {
		return nil, err
	}
	requestID, err := parseRequestID(raw)
	if err != nil {
		return nil, err
	}
	return b.newPollingResponse(requestID), nil
}

func (b *backend) GetTaskStatus(ctx context.Context, requestID string) (xai.OperationResponse, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, fmt.Errorf("qiniu-minimax-h3: request_id is empty")
	}
	statusRaw, err := b.client.GetJSON(ctx, statusEndpoint(requestID))
	if err != nil {
		return nil, err
	}
	status, detail, err := parseStatus(statusRaw)
	if err != nil {
		return nil, err
	}
	switch strings.ToUpper(status) {
	case StatusCompleted:
		if detailMessage(detail) != "" {
			return nil, fmt.Errorf("%w: %s", ErrTaskFailed, detailMessage(detail))
		}
		resultRaw, err := b.client.GetJSON(ctx, resultEndpoint(requestID))
		if err != nil {
			return nil, err
		}
		videoURL, err := parseVideoURL(resultRaw)
		if err != nil {
			return nil, err
		}
		return &minimax_h3.SyncOperationResponse{R: minimax_h3.NewOutputVideos([]string{videoURL})}, nil
	case StatusFailed, StatusCancelled, StatusCanceled:
		message := detailMessage(detail)
		if message == "" {
			message = strings.ToLower(status)
		}
		return nil, fmt.Errorf("%w: %s", ErrTaskFailed, message)
	default:
		return b.newPollingResponse(requestID), nil
	}
}

func (b *backend) newPollingResponse(requestID string) xai.OperationResponse {
	return minimax_h3.NewAsyncOperationResponse(func(ctx context.Context) (xai.OperationResponse, error) {
		return b.GetTaskStatus(ctx, requestID)
	}, requestID)
}

func statusEndpoint(requestID string) string {
	return EndpointStatusPrefix + url.PathEscape(requestID) + "/status"
}

func resultEndpoint(requestID string) string {
	return EndpointStatusPrefix + url.PathEscape(requestID)
}

func parseRequestID(raw []byte) (string, error) {
	var response struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", fmt.Errorf("qiniu-minimax-h3: parse create response: %w", err)
	}
	if strings.TrimSpace(response.RequestID) == "" {
		return "", fmt.Errorf("qiniu-minimax-h3: create response missing request_id")
	}
	return strings.TrimSpace(response.RequestID), nil
}

func parseStatus(raw []byte) (string, json.RawMessage, error) {
	var response struct {
		Status string          `json:"status"`
		Detail json.RawMessage `json:"detail"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", nil, fmt.Errorf("qiniu-minimax-h3: parse status response: %w", err)
	}
	if strings.TrimSpace(response.Status) == "" {
		return "", nil, fmt.Errorf("qiniu-minimax-h3: status response missing status")
	}
	return strings.TrimSpace(response.Status), response.Detail, nil
}

func detailMessage(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var detail struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal(raw, &detail) == nil {
		if strings.TrimSpace(detail.Message) != "" {
			return strings.TrimSpace(detail.Message)
		}
		if strings.TrimSpace(detail.Detail) != "" {
			return strings.TrimSpace(detail.Detail)
		}
		if strings.TrimSpace(detail.Type) != "" {
			return strings.TrimSpace(detail.Type)
		}
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(string(raw))
}

func parseVideoURL(raw []byte) (string, error) {
	var response struct {
		Video struct {
			URL string `json:"url"`
		} `json:"video"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", fmt.Errorf("qiniu-minimax-h3: parse result response: %w", err)
	}
	if strings.TrimSpace(response.Video.URL) == "" {
		return "", fmt.Errorf("%w: completed result missing video.url", ErrTaskFailed)
	}
	return strings.TrimSpace(response.Video.URL), nil
}

/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package qiniu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	xai "github.com/goplus/xai/spec"
	"github.com/goplus/xai/spec/seedance"
	seedanceassets "github.com/goplus/xai/spec/seedance_assets"
)

const pathCreateTask = "/v3/contents/generations/tasks"

const (
	defaultAssetsBaseURL = "https://openai.qiniu.com"

	ParamAssetGroupID       = "asset_group_id"
	ParamAssetAutoReview    = "asset_auto_review"
	ParamAssetPollInterval  = "asset_poll_interval"
	ParamAssetPollAttempts  = "asset_poll_attempts"
	ParamAssetReviewRetries = "asset_review_retries"
)

// ErrTaskFailed is returned when Qiniu reports a terminal failure.
var ErrTaskFailed = errors.New("qiniu-seedance: task failed")

type backend struct {
	client      *Client
	assetClient *Client
}

func newBackend(client *Client) *backend {
	return &backend{
		client:      client,
		assetClient: newAssetClient(client),
	}
}

// NewBackend returns a seedance.Backend backed by the Qiniu/Qnagic HTTP API.
func NewBackend(client *Client) seedance.Backend {
	return newBackend(client)
}

// Submit creates an async generation task and returns a polling OperationResponse.
func (b *backend) Submit(ctx context.Context, model xai.Model, params xai.Params) (xai.OperationResponse, error) {
	p, ok := params.(*seedance.Params)
	if !ok {
		return nil, fmt.Errorf("qiniu-seedance: expected *seedance.Params, got %T", params)
	}
	m := strings.TrimSpace(string(model))
	b.client.LogDebug("Submit GenVideo model=%q", m)

	body, err := b.buildTaskBody(ctx, m, p)
	if err != nil {
		b.client.LogDebug("Submit buildTaskBody error: %v", err)
		return nil, err
	}
	raw, err := b.client.PostJSON(ctx, pathCreateTask, body)
	if err != nil {
		b.client.LogDebug("Submit POST error: %v", err)
		return nil, err
	}
	taskID, err := parseCreateTaskID(raw)
	if err != nil {
		b.client.LogDebug("Submit parse task id error: %v", err)
		return nil, err
	}
	b.client.LogDebug("Submit created task_id=%q (async polling)", taskID)
	return b.newPollingResponse(taskID), nil
}

// GetTaskStatus loads task state and converts terminal success/failure into xai.OperationResponse.
func (b *backend) GetTaskStatus(ctx context.Context, taskID string) (xai.OperationResponse, error) {
	b.client.LogDebug("GetTaskStatus task_id=%q", taskID)
	raw, err := b.getTaskJSON(ctx, taskID)
	if err != nil {
		b.client.LogDebug("GetTaskStatus request error: %v", err)
		return nil, err
	}
	status, videoURL, failMsg, err := parseTaskGetResponse(raw)
	if err != nil {
		b.client.LogDebug("GetTaskStatus parse error: %v", err)
		return nil, err
	}
	st := normalizeStatus(status)
	b.client.LogDebug("GetTaskStatus task_id=%q raw_status=%q normalized=%q video_url_present=%v", taskID, status, st, videoURL != "")

	switch {
	case isSuccessStatus(st):
		if videoURL == "" {
			return nil, fmt.Errorf("%w: succeeded but no video_url in response", ErrTaskFailed)
		}
		return &seedance.SyncOperationResponse{R: seedance.NewOutputVideos([]string{videoURL})}, nil
	case isFailedStatus(st):
		if failMsg == "" {
			failMsg = "unknown error"
		}
		return nil, fmt.Errorf("%w: %s", ErrTaskFailed, failMsg)
	default:
		return b.newPollingResponse(taskID), nil
	}
}

func (b *backend) newPollingResponse(taskID string) xai.OperationResponse {
	return seedance.NewAsyncOperationResponse(func(ctx context.Context) (xai.OperationResponse, error) {
		return b.GetTaskStatus(ctx, taskID)
	}, taskID)
}

func (b *backend) getTaskJSON(ctx context.Context, taskID string) ([]byte, error) {
	path := pathCreateTask + "/" + url.PathEscape(taskID)
	return b.client.GetJSON(ctx, path)
}

func (b *backend) buildTaskBody(ctx context.Context, model string, p *seedance.Params) (map[string]any, error) {
	m := normalizeQiniuModel(model)
	if m == "" {
		return nil, fmt.Errorf("qiniu-seedance: empty model")
	}

	text := p.PrimaryText()
	if text == "" {
		return nil, seedance.ErrTextRequired
	}

	content := []any{
		map[string]any{
			"type": "text",
			"text": text,
		},
	}

	explicitRefs := p.GetReferenceImages(seedance.ParamReferenceImages)
	if len(explicitRefs) > 0 {
		for _, ref := range explicitRefs {
			role := strings.TrimSpace(ref.Role)
			if role == "" {
				role = "reference_image"
			}
			u, err := b.prepareAssetURL(ctx, p, ref.URL, "image", "参考图", m)
			if err != nil {
				return nil, err
			}
			content = append(content, imageURLContent(u, role))
		}
	} else {
		for _, refURL := range p.GetStringSlice(seedance.ParamReferenceImageURLs) {
			u, err := b.prepareAssetURL(ctx, p, refURL, "image", "参考图", m)
			if err != nil {
				return nil, err
			}
			content = append(content, imageURLContent(u, "reference_image"))
		}
	}
	for _, refURL := range p.GetStringSlice(seedance.ParamReferenceVideoURLs) {
		u, err := b.prepareAssetURL(ctx, p, refURL, "video", "参考视频", m)
		if err != nil {
			return nil, err
		}
		content = append(content, mediaURLContent("video_url", "video_url", u, "reference_video"))
	}
	for _, refURL := range p.GetStringSlice(seedance.ParamReferenceAudioURLs) {
		u, err := b.prepareAssetURL(ctx, p, refURL, "audio", "参考音频", m)
		if err != nil {
			return nil, err
		}
		content = append(content, mediaURLContent("audio_url", "audio_url", u, "reference_audio"))
	}

	body := map[string]any{
		"model":   m,
		"content": content,
	}

	if d := p.GetInt(seedance.ParamDuration); d != nil {
		if err := seedance.ValidateVideoDuration(m, *d); err != nil {
			return nil, err
		}
		body["duration"] = *d
	}
	if resolution := p.GetString("resolution"); resolution != "" {
		body["resolution"] = resolution
	}
	if r := p.GetString(seedance.ParamRatio); r != "" {
		body["ratio"] = r
	}
	if ga := p.GetBool(seedance.ParamGenerateAudio); ga != nil {
		body["generate_audio"] = *ga
	}
	if autoCreateAssets := p.GetBool(seedance.ParamAutoCreateAssets); autoCreateAssets != nil {
		body["auto_create_assets"] = *autoCreateAssets
	}
	return body, nil
}

func buildTaskBody(model string, p *seedance.Params) (map[string]any, error) {
	return (&backend{}).buildTaskBody(context.Background(), model, p)
}

func (b *backend) prepareAssetURL(ctx context.Context, p *seedance.Params, rawURL, typ, name, model string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || strings.HasPrefix(strings.ToLower(rawURL), "qasset://") {
		return rawURL, nil
	}
	if autoCreateAssets := p.GetBool(seedance.ParamAutoCreateAssets); autoCreateAssets != nil && *autoCreateAssets {
		return rawURL, nil
	}
	if isQiniuModelWithoutAssets(model) {
		return rawURL, nil
	}
	if v := p.GetBool(ParamAssetAutoReview); v != nil && !*v {
		return rawURL, nil
	}
	if b == nil || b.assetClient == nil {
		return rawURL, nil
	}

	req := &seedanceassets.UploadAssetRequest{
		GroupID:   assetGroupID(p),
		Name:      name,
		AssetType: typ,
		URL:       rawURL,
		Model:     model,
	}

	retries := assetReviewRetries(p)
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			b.client.LogDebug("retry asset review url=%q attempt=%d/%d after error: %v", rawURL, attempt+1, retries+1, lastErr)
		}
		asset, err := b.createAndAwaitAsset(ctx, req, assetPollInterval(p), assetPollAttempts(p))
		if err == nil {
			if strings.TrimSpace(asset.ID) == "" {
				return "", fmt.Errorf("qiniu-seedance: asset id is empty after review")
			}
			return "qasset://" + strings.TrimSpace(asset.ID), nil
		}
		lastErr = err
		if attempt >= retries || !isRetryableAssetReviewError(err) {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(assetPollInterval(p)):
		}
	}
	return "", lastErr
}

func (b *backend) createAndAwaitAsset(ctx context.Context, req *seedanceassets.UploadAssetRequest, interval time.Duration, attempts int) (*seedanceassets.Asset, error) {
	ret, err := b.createAsset(ctx, req)
	if err != nil {
		return nil, err
	}
	return b.awaitAsset(ctx, ret.AssetID, interval, attempts)
}

func (b *backend) createAsset(ctx context.Context, req *seedanceassets.UploadAssetRequest) (*seedanceassets.AssetUploadResult, error) {
	body := map[string]any{
		"type":  normalizeAssetType(req.AssetType),
		"url":   strings.TrimSpace(req.URL),
		"name":  strings.TrimSpace(req.Name),
		"model": strings.TrimSpace(req.Model),
	}
	if groupID := strings.TrimSpace(req.GroupID); groupID != "" {
		body["group_id"] = groupID
	}
	raw, err := b.assetClient.PostJSON(ctx, "/v1/assets", body)
	if err != nil {
		return nil, err
	}
	var v struct {
		ID         string `json:"qassetid"`
		Type       string `json:"type"`
		Name       string `json:"name"`
		Model      string `json:"model"`
		Status     string `json:"status"`
		GroupID    string `json:"group_id"`
		FailReason string `json:"fail_reason"`
		CreatedAt  int64  `json:"created_at"`
		UpdatedAt  int64  `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("qiniu-seedance: parse create asset response: %w", err)
	}
	return &seedanceassets.AssetUploadResult{
		AssetID:    strings.TrimSpace(v.ID),
		AssetType:  strings.TrimSpace(v.Type),
		Name:       strings.TrimSpace(v.Name),
		Model:      strings.TrimSpace(v.Model),
		Status:     strings.TrimSpace(v.Status),
		GroupID:    strings.TrimSpace(v.GroupID),
		FailReason: strings.TrimSpace(v.FailReason),
		CreatedAt:  v.CreatedAt,
		UpdatedAt:  v.UpdatedAt,
	}, nil
}

func (b *backend) getAsset(ctx context.Context, assetID string) (*seedanceassets.Asset, error) {
	assetID = strings.TrimSpace(assetID)
	if assetID == "" {
		return nil, fmt.Errorf("qiniu-seedance: qassetid is required")
	}
	raw, err := b.assetClient.GetJSON(ctx, "/v1/assets/"+url.PathEscape(assetID))
	if err != nil {
		return nil, err
	}
	var v struct {
		ID         string `json:"qassetid"`
		Type       string `json:"type"`
		Name       string `json:"name"`
		Model      string `json:"model"`
		Status     string `json:"status"`
		GroupID    string `json:"group_id"`
		FailReason string `json:"fail_reason"`
		CreatedAt  int64  `json:"created_at"`
		UpdatedAt  int64  `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("qiniu-seedance: parse get asset response: %w", err)
	}
	return &seedanceassets.Asset{
		ID:         strings.TrimSpace(v.ID),
		GroupID:    strings.TrimSpace(v.GroupID),
		Name:       strings.TrimSpace(v.Name),
		Type:       strings.TrimSpace(v.Type),
		Model:      strings.TrimSpace(v.Model),
		Status:     strings.TrimSpace(v.Status),
		FailReason: strings.TrimSpace(v.FailReason),
		CreatedAt:  v.CreatedAt,
		UpdatedAt:  v.UpdatedAt,
	}, nil
}

func (b *backend) awaitAsset(ctx context.Context, assetID string, interval time.Duration, attempts int) (*seedanceassets.Asset, error) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if attempts <= 0 {
		attempts = 40
	}
	for i := 0; i < attempts; i++ {
		asset, err := b.getAsset(ctx, assetID)
		if err != nil {
			return nil, err
		}
		status := strings.ToLower(strings.TrimSpace(asset.Status))
		b.client.LogDebug("asset review qassetid=%q status=%q", asset.ID, asset.Status)
		switch status {
		case "approved":
			return asset, nil
		case "failed":
			reason := strings.TrimSpace(asset.FailReason)
			if reason == "" {
				reason = "unknown error"
			}
			return nil, fmt.Errorf("qiniu-seedance: asset review failed: %s", reason)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
	return nil, fmt.Errorf("qiniu-seedance: asset review timeout: %s", assetID)
}

func assetGroupID(p *seedance.Params) string {
	if p != nil {
		if groupID := strings.TrimSpace(p.GetString(ParamAssetGroupID)); groupID != "" {
			return groupID
		}
	}
	return strings.TrimSpace(os.Getenv("QINIU_ASSET_GROUP_ID"))
}

func assetPollInterval(p *seedance.Params) time.Duration {
	if p != nil {
		if ms := p.GetInt(ParamAssetPollInterval); ms != nil && *ms > 0 {
			return time.Duration(*ms) * time.Millisecond
		}
	}
	return 2 * time.Second
}

func assetPollAttempts(p *seedance.Params) int {
	if p != nil {
		if n := p.GetInt(ParamAssetPollAttempts); n != nil && *n > 0 {
			return *n
		}
	}
	return 40
}

func assetReviewRetries(p *seedance.Params) int {
	if p != nil {
		if n := p.GetInt(ParamAssetReviewRetries); n != nil && *n >= 0 {
			return *n
		}
	}
	return 2
}

func isRetryableAssetReviewError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"infra",
		"asset api",
		"timeout",
		"temporary",
		"temporarily",
		"connection",
		"request failed",
		"http 429",
		"http 500",
		"http 502",
		"http 503",
		"http 504",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func normalizeAssetType(typ string) string {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "image":
		return "image"
	case "video":
		return "video"
	case "audio":
		return "audio"
	default:
		return strings.TrimSpace(typ)
	}
}

func newAssetClient(client *Client) *Client {
	apiKey := ""
	debugLog := true
	logger := log.Default()
	if client != nil {
		apiKey = client.ApiKey()
		debugLog = client.debugLog
		logger = client.logger
	}
	baseURL := strings.TrimSpace(os.Getenv("QINIU_ASSETS_BASE_URL"))
	if baseURL == "" {
		baseURL = defaultAssetsBaseURL
	}
	return NewClient(apiKey, WithBaseURL(baseURL), WithDebugLog(debugLog), WithLogger(logger))
}

func normalizeQiniuModel(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return ""
	}
	if isQiniuModelWithoutAssets(m) {
		return m
	}
	if strings.Contains(m, "/") {
		return m
	}
	if strings.HasPrefix(strings.ToLower(m), "doubao-seedance-") {
		return "bytedance/" + m
	}
	return m
}

func isQiniuModelWithoutAssets(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), seedance.ModelByteplusDreaminaSeedance20)
}

func imageURLContent(u string, role string) map[string]any {
	return mediaURLContent("image_url", "image_url", u, role)
}

func mediaURLContent(contentType string, field string, u string, role string) map[string]any {
	return map[string]any{
		"type": contentType,
		field: map[string]any{
			"url": u,
		},
		"role": role,
	}
}

func parseCreateTaskID(raw []byte) (string, error) {
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", fmt.Errorf("qiniu-seedance: parse create response: %w", err)
	}
	if id := stringVal(v, "id"); id != "" {
		return id, nil
	}
	if d, ok := v["data"].(map[string]any); ok {
		if id := stringVal(d, "id"); id != "" {
			return id, nil
		}
		if id := stringVal(d, "task_id"); id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("qiniu-seedance: no task id in create response")
}

func parseTaskGetResponse(raw []byte) (status, videoURL, failMsg string, err error) {
	var v map[string]any
	if err = json.Unmarshal(raw, &v); err != nil {
		return "", "", "", fmt.Errorf("qiniu-seedance: parse get task response: %w", err)
	}
	status = firstStatus(v)
	videoURL = findVideoURL(v)
	failMsg = firstErrorMessage(v)
	if d, ok := v["data"].(map[string]any); ok {
		if status == "" {
			status = firstStatus(d)
		}
		if videoURL == "" {
			videoURL = findVideoURL(d)
		}
		if failMsg == "" {
			failMsg = firstErrorMessage(d)
		}
	}
	return status, videoURL, failMsg, nil
}

func firstStatus(m map[string]any) string {
	return stringVal(m, "status")
}

func firstErrorMessage(m map[string]any) string {
	if s := stringVal(m, "message"); s != "" {
		return s
	}
	if s := stringVal(m, "error"); s != "" {
		return s
	}
	if em, ok := m["error"].(map[string]any); ok {
		return stringVal(em, "message")
	}
	return ""
}

func findVideoURL(m map[string]any) string {
	if s := stringVal(m, "video_url"); s != "" {
		return s
	}
	if c, ok := m["content"].(map[string]any); ok {
		if s := stringVal(c, "video_url"); s != "" {
			return s
		}
		if nested, ok := c["video_url"].(map[string]any); ok {
			if s := stringVal(nested, "url"); s != "" {
				return s
			}
		}
	}
	if arr, ok := m["content"].([]any); ok {
		for _, item := range arr {
			im, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if s := stringVal(im, "video_url"); s != "" {
				return s
			}
			if nested, ok := im["video_url"].(map[string]any); ok {
				if s := stringVal(nested, "url"); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func stringVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func normalizeStatus(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func isSuccessStatus(s string) bool {
	switch s {
	case "succeeded", "success", "completed", "complete", "done":
		return true
	default:
		return false
	}
}

func isFailedStatus(s string) bool {
	switch s {
	case "failed", "error", "cancelled", "canceled", "expired":
		return true
	default:
		return false
	}
}

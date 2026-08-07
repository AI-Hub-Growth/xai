/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 */

package minimax_h3

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	xai "github.com/goplus/xai/spec"
)

var (
	ErrPromptRequired   = fmt.Errorf("minimax-h3: prompt is required")
	ErrImageRequired    = fmt.Errorf("minimax-h3: image_url is required")
	ErrEndImageRequired = fmt.Errorf("minimax-h3: end_image_url is required")
)

type serviceProvider interface {
	MiniMaxH3Service() *Service
}

func (p *Service) Actions(model xai.Model) []xai.Action {
	if IsVideoModel(string(model)) {
		return []xai.Action{xai.GenVideo}
	}
	return nil
}

func (p *Service) Operation(model xai.Model, action xai.Action) (xai.Operation, error) {
	if action != xai.GenVideo || !IsVideoModel(string(model)) {
		return nil, xai.ErrNotFound
	}
	return &genVideo{model: strings.TrimSpace(string(model))}, nil
}

type genVideo struct {
	model  string
	params *Params
}

func (p *genVideo) InputSchema() xai.InputSchema { return &inputSchema{} }

func (p *genVideo) Params() xai.Params {
	if p.params == nil {
		p.params = NewParams()
	}
	return p.params
}

func (p *genVideo) Call(ctx context.Context, svc xai.Service, opts xai.OptionBuilder) (xai.OperationResponse, error) {
	provider, ok := svc.(serviceProvider)
	if !ok {
		return nil, xai.ErrNotFound
	}
	h3 := provider.MiniMaxH3Service()
	if h3 == nil || h3.backend == nil {
		return nil, xai.ErrNotFound
	}
	params := p.Params().(*Params)
	if err := validateParams(params); err != nil {
		return nil, err
	}
	_ = opts
	return h3.backend.Submit(ctx, xai.Model(p.model), params)
}

type inputSchema struct{}

func (*inputSchema) Fields() []xai.Field                   { return GenVideoFields() }
func (*inputSchema) Restrict(name string) *xai.Restriction { return GenVideoRestrict(name) }

func validateParams(params *Params) error {
	prompt := params.GetString(ParamPrompt)
	if prompt == "" {
		return ErrPromptRequired
	}
	if utf8.RuneCountInString(prompt) > 7000 {
		return fmt.Errorf("minimax-h3: prompt exceeds 7000 characters")
	}
	if resolution := params.GetString(ParamResolution); resolution != "" && !containsFold(allowedResolutions, resolution) {
		return fmt.Errorf("minimax-h3: unsupported resolution %q", resolution)
	}
	if duration := params.GetInt(ParamDuration); duration != nil && (*duration < 5 || *duration > 15) {
		return fmt.Errorf("minimax-h3: duration must be between 5 and 15 seconds")
	}
	mode := normalizeVideoMode(params)
	if aspect := params.GetString(ParamAspectRatio); aspect != "" && !IsAspectRatioAllowed(mode, aspect) {
		return fmt.Errorf("minimax-h3: unsupported aspect_ratio %q for mode %q", aspect, mode)
	}
	switch mode {
	case VideoModeTextToVideo:
	case VideoModeImageToVideo:
		if !params.HasNonEmptyString(ParamImageURL) {
			return ErrImageRequired
		}
	case VideoModeStartEndToVideo:
		if !params.HasNonEmptyString(ParamImageURL) {
			return ErrImageRequired
		}
		if !params.HasNonEmptyString(ParamEndImageURL) {
			return ErrEndImageRequired
		}
	case VideoModeMultiRefToVideo:
		images := params.GetStringSlice(ParamReferenceImageURLs)
		videos := params.GetStringSlice(ParamReferenceVideoURLs)
		audios := params.GetStringSlice(ParamReferenceAudioURLs)
		if len(images) == 0 && len(videos) == 0 {
			return fmt.Errorf("minimax-h3: multi-reference mode requires an image or video")
		}
		if len(images) > 9 || len(videos) > 3 || len(audios) > 3 || len(images)+len(videos)+len(audios) > 12 {
			return fmt.Errorf("minimax-h3: reference materials exceed the supported limit")
		}
	default:
		return fmt.Errorf("minimax-h3: unsupported video mode %q", mode)
	}
	return nil
}

// ValidateParams validates the protocol-level H3 input rules. Provider
// backends can call it when they are used directly in tests or integrations.
func ValidateParams(params *Params) error {
	if params == nil {
		return fmt.Errorf("minimax-h3: params is nil")
	}
	return validateParams(params)
}

// VideoMode returns the explicit mode or infers one from the supplied media.
func VideoMode(params *Params) string {
	if params == nil {
		return VideoModeTextToVideo
	}
	return normalizeVideoMode(params)
}

func containsFold(values []string, value string) bool {
	for _, candidate := range values {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

// GetTask resumes polling for an existing request ID.
func (p *Service) GetTask(ctx context.Context, _ xai.Model, action xai.Action, requestID string) (xai.OperationResponse, error) {
	if action != xai.GenVideo || p.backend == nil || strings.TrimSpace(requestID) == "" {
		return nil, xai.ErrNotFound
	}
	return p.backend.GetTaskStatus(ctx, strings.TrimSpace(requestID))
}

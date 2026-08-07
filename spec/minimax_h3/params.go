/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 */

package minimax_h3

import (
	"strconv"
	"strings"

	xai "github.com/goplus/xai/spec"
)

// Parameter names mirror the MiniMax-H3 FAL request schemas.
const (
	ParamPrompt             = "prompt"
	ParamImageURL           = "image_url"
	ParamEndImageURL        = "end_image_url"
	ParamReferenceImageURLs = "reference_image_urls"
	ParamReferenceVideoURLs = "reference_video_urls"
	ParamReferenceAudioURLs = "reference_audio_urls"
	ParamDuration           = "duration"
	ParamResolution         = "resolution"
	ParamAspectRatio        = "aspect_ratio"
	ParamVideoMode          = "video_mode"
)

// Video mode values used to choose one of the three upstream endpoints.
const (
	VideoModeTextToVideo     = "text_to_video"
	VideoModeImageToVideo    = "image_to_video"
	VideoModeStartEndToVideo = "start_end_to_video"
	VideoModeMultiRefToVideo = "multi_ref_to_video"
)

// Params stores MiniMax-H3 operation inputs.
type Params struct {
	m map[string]any
}

// NewParams creates an empty parameter set.
func NewParams() *Params { return &Params{m: make(map[string]any)} }

// Set implements xai.Params.
func (p *Params) Set(name string, value any) xai.Params {
	if p.m == nil {
		p.m = make(map[string]any)
	}
	p.m[name] = value
	return p
}

// Export returns a shallow copy of the parameters.
func (p *Params) Export() map[string]any {
	out := make(map[string]any, len(p.m))
	for key, value := range p.m {
		out[key] = value
	}
	return out
}

// Get returns a raw parameter.
func (p *Params) Get(name string) (any, bool) { value, ok := p.m[name]; return value, ok }

// GetString returns a trimmed string or an empty string.
func (p *Params) GetString(name string) string {
	value, ok := p.m[name]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

// GetInt returns an integer parameter when it can be decoded.
func (p *Params) GetInt(name string) *int {
	value, ok := p.m[name]
	if !ok {
		return nil
	}
	var result int
	switch value := value.(type) {
	case int:
		result = value
	case int64:
		result = int(value)
	case float64:
		result = int(value)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return nil
		}
		result = parsed
	default:
		return nil
	}
	return &result
}

// GetStringSlice accepts []string, []any of strings, or one string.
func (p *Params) GetStringSlice(name string) []string {
	value, ok := p.m[name]
	if !ok {
		return nil
	}
	var values []string
	switch value := value.(type) {
	case string:
		values = []string{value}
	case []string:
		values = append(values, value...)
	case []any:
		for _, item := range value {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
	default:
		return nil
	}
	out := make([]string, 0, len(values))
	for _, item := range values {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// HasNonEmptyString reports whether name contains a non-empty string.
func (p *Params) HasNonEmptyString(name string) bool { return p.GetString(name) != "" }

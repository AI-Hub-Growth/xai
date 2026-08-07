/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 */

package minimax_h3

import (
	"strings"

	xai "github.com/goplus/xai/spec"
	"github.com/goplus/xai/types"
)

var (
	allowedResolutions  = []string{"2K", "768P"}
	allowedAspectRatios = []string{
		"adaptive", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16",
	}
)

// GenVideoFields returns the union of MiniMax-H3 request fields.
func GenVideoFields() []xai.Field {
	return []xai.Field{
		{Name: ParamPrompt, Kind: types.String},
		{Name: ParamImageURL, Kind: types.String},
		{Name: ParamEndImageURL, Kind: types.String},
		{Name: ParamReferenceImageURLs, Kind: types.String | types.List},
		{Name: ParamReferenceVideoURLs, Kind: types.String | types.List},
		{Name: ParamReferenceAudioURLs, Kind: types.String | types.List},
		{Name: ParamDuration, Kind: types.Int},
		{Name: ParamResolution, Kind: types.String},
		{Name: ParamAspectRatio, Kind: types.String},
	}
}

// GenVideoRestrict returns enum and required-field restrictions used by the
// catalog. Mode-dependent required checks are performed by the operation.
func GenVideoRestrict(name string) *xai.Restriction {
	switch name {
	case ParamPrompt:
		return &xai.Restriction{Required: true}
	case ParamResolution:
		return &xai.Restriction{Limit: &xai.StringEnum{Values: allowedResolutions}}
	case ParamAspectRatio:
		return &xai.Restriction{Limit: &xai.StringEnum{Values: allowedAspectRatios}}
	case ParamDuration:
		values := make([]int64, 0, 11)
		for value := int64(5); value <= 15; value++ {
			values = append(values, value)
		}
		return &xai.Restriction{Limit: &xai.IntEnum{Values: values}}
	default:
		return nil
	}
}

// VideoSchemaFor returns the MiniMax-H3 video schema for model.
func VideoSchemaFor(model string) xai.VideoSchema {
	if !IsVideoModel(model) {
		return nil
	}
	return &videoSchema{}
}

type videoSchema struct{}

func (*videoSchema) SupportedModes() []xai.VideoGenMode {
	return []xai.VideoGenMode{
		xai.VideoGenModeText,
		xai.VideoGenModeImage,
		xai.VideoGenModeStartEnd,
		xai.VideoGenModeMultiRef,
	}
}

func (*videoSchema) Fields() []xai.Field { return GenVideoFields() }

func (*videoSchema) Restrict(name string) *xai.Restriction { return GenVideoRestrict(name) }

func (*videoSchema) FieldModes(name string) []xai.VideoGenMode {
	switch name {
	case ParamImageURL:
		return []xai.VideoGenMode{xai.VideoGenModeImage, xai.VideoGenModeStartEnd}
	case ParamEndImageURL:
		return []xai.VideoGenMode{xai.VideoGenModeStartEnd}
	case ParamReferenceImageURLs, ParamReferenceVideoURLs, ParamReferenceAudioURLs:
		return []xai.VideoGenMode{xai.VideoGenModeMultiRef}
	default:
		return nil
	}
}

func normalizeVideoMode(params *Params) string {
	if mode := strings.TrimSpace(params.GetString(ParamVideoMode)); mode != "" {
		return mode
	}
	if len(params.GetStringSlice(ParamReferenceImageURLs)) > 0 ||
		len(params.GetStringSlice(ParamReferenceVideoURLs)) > 0 ||
		len(params.GetStringSlice(ParamReferenceAudioURLs)) > 0 {
		return VideoModeMultiRefToVideo
	}
	if params.HasNonEmptyString(ParamImageURL) {
		return VideoModeImageToVideo
	}
	return VideoModeTextToVideo
}

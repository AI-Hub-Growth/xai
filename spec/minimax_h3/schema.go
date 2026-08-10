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
	allowedResolutions      = []string{"2K", "768P"}
	allowedTextAspectRatios = []string{
		"21:9", "16:9", "4:3", "1:1", "3:4", "9:16",
	}
	allowedReferenceAspectRatios = append([]string{"adaptive"}, allowedTextAspectRatios...)
	// The catalog restriction is a union because the current xai schema contract
	// exposes one restriction per field. Runtime validation below applies the
	// narrower mode-specific set.
	allowedAspectRatios = append([]string(nil), allowedReferenceAspectRatios...)
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

// AspectRatiosForMode returns the upstream-supported aspect ratios for a H3
// generation mode. Image-to-video and start/end-to-video do not accept an
// aspect ratio because their output follows the input image.
func AspectRatiosForMode(mode string) []string {
	switch strings.TrimSpace(mode) {
	case VideoModeTextToVideo:
		return append([]string(nil), allowedTextAspectRatios...)
	case VideoModeMultiRefToVideo:
		return append([]string(nil), allowedReferenceAspectRatios...)
	default:
		return nil
	}
}

// IsAspectRatioAllowed reports whether aspect is valid for the selected H3
// mode. An empty aspect is allowed because the upstream default applies.
func IsAspectRatioAllowed(mode, aspect string) bool {
	if strings.TrimSpace(aspect) == "" {
		return true
	}
	return containsFold(AspectRatiosForMode(mode), aspect)
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
	case ParamAspectRatio:
		return []xai.VideoGenMode{xai.VideoGenModeText, xai.VideoGenModeMultiRef}
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
	if params.HasNonEmptyString(ParamImageURL) && params.HasNonEmptyString(ParamEndImageURL) {
		return VideoModeStartEndToVideo
	}
	if params.HasNonEmptyString(ParamImageURL) {
		return VideoModeImageToVideo
	}
	return VideoModeTextToVideo
}

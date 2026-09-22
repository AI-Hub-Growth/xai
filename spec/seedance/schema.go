/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package seedance

import (
	xai "github.com/goplus/xai/spec"
	"github.com/goplus/xai/types"
)

// GenVideoFields returns InputSchema fields for Seedance GenVideo.
func GenVideoFields() []xai.Field {
	return []xai.Field{
		{Name: ParamText, Kind: types.String},
		{Name: ParamPrompt, Kind: types.String},
		{Name: ParamReferenceImageURLs, Kind: types.String},
		{Name: ParamReferenceImages, Kind: types.List},
		{Name: ParamReferenceVideoURLs, Kind: types.String},
		{Name: ParamReferenceAudioURLs, Kind: types.String},
		{Name: ParamDuration, Kind: types.Int},
		{Name: ParamRatio, Kind: types.String},
		{Name: ParamOmniReferenceTaskType, Kind: types.String},
		{Name: ParamGenerateAudio, Kind: types.Bool},
		{Name: ParamWatermark, Kind: types.Bool},
		{Name: ParamAutoCreateAssets, Kind: types.Bool},
	}
}

// GenVideoRestrict returns restrictions for known fields.
// Ratio and other Ark fields follow the product matrix in the official doc; no hard enum here so new doc values are not rejected by the spec layer.
func GenVideoRestrict(name string) *xai.Restriction {
	switch name {
	case ParamText, ParamPrompt:
		return nil // either text or prompt required; validated in operation
	default:
		return nil
	}
}

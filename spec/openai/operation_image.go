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

package openai

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	xai "github.com/goplus/xai/spec"
	"github.com/goplus/xai/spec/util"
	"github.com/goplus/xai/types"
)

const (
	imageGenerationsEndpoint = "images/generations"
	imageEditsEndpoint       = "images/edits"
)

var (
	enumImageQuality = &xai.StringEnum{
		Values: []string{"low", "medium", "high", "auto"},
	}
	enumGPTImage25Quality = &xai.StringEnum{
		Values: []string{"low", "medium", "high", "xhigh", "max", "auto"},
	}
	imageRestrictions = map[string]*xai.Restriction{
		ParamPrompt:  {Required: true},
		ParamQuality: {Limit: enumImageQuality},
		ParamImages:  {Required: true},
	}
	imageFields = []xai.Field{
		{Name: ParamPrompt, Kind: types.String},
		{Name: ParamSize, Kind: types.String},
		{Name: ParamQuality, Kind: types.String},
		{Name: ParamImage, Kind: types.String | types.Image},
		{Name: ParamImages, Kind: types.List},
	}
)

type imageInputSchema struct {
	model string
	edit  bool
}

func (s imageInputSchema) Fields() []xai.Field {
	if s.edit {
		return []xai.Field{
			{Name: ParamPrompt, Kind: types.String},
			{Name: ParamSize, Kind: types.String},
			{Name: ParamQuality, Kind: types.String},
			{Name: ParamImages, Kind: types.List},
		}
	}
	return []xai.Field{
		{Name: ParamPrompt, Kind: types.String},
		{Name: ParamSize, Kind: types.String},
		{Name: ParamQuality, Kind: types.String},
		{Name: ParamImage, Kind: types.String | types.Image},
	}
}

func (s imageInputSchema) Restrict(name string) *xai.Restriction {
	if name == ParamQuality {
		return imageQualityRestriction(s.model)
	}
	if s.edit && name == ParamImages {
		return imageRestrictions[name]
	}
	if !s.edit && name == ParamImage {
		return nil
	}
	return imageRestrictions[name]
}

type genImage struct {
	model  string
	params *imageParams
}

func (p *genImage) InputSchema() xai.InputSchema {
	return imageInputSchema{model: p.model}
}

func (p *genImage) Params() xai.Params {
	if p.params == nil {
		p.params = &imageParams{}
	}
	return p.params
}

func (p *genImage) Call(ctx context.Context, svc xai.Service, opts xai.OptionBuilder) (xai.OperationResponse, error) {
	s, ok := svc.(*Service)
	if !ok {
		return nil, xai.ErrNotSupported
	}
	params := p.Params().(*imageParams)
	if err := params.validate(p.model, false); err != nil {
		return nil, err
	}

	body := map[string]any{
		"model":  p.model,
		"prompt": params.Prompt,
	}
	if params.Quality != "" {
		body["quality"] = params.Quality
	}
	if params.Size != "" {
		body["size"] = params.Size
	}
	if params.Image != "" {
		body["image"] = params.Image
	}

	baseURL := s.operationBaseURL(opts)
	resp, err := s.postImageRequest(ctx, baseURL, imageGenerationsEndpoint, body)
	if err != nil {
		return nil, err
	}
	return util.NewSimpleResp(newImageResults(resp)), nil
}

type editImage struct {
	model  string
	params *imageParams
}

func (p *editImage) InputSchema() xai.InputSchema {
	return imageInputSchema{model: p.model, edit: true}
}

func (p *editImage) Params() xai.Params {
	if p.params == nil {
		p.params = &imageParams{}
	}
	return p.params
}

func (p *editImage) Call(ctx context.Context, svc xai.Service, opts xai.OptionBuilder) (xai.OperationResponse, error) {
	s, ok := svc.(*Service)
	if !ok {
		return nil, xai.ErrNotSupported
	}
	params := p.Params().(*imageParams)
	if err := params.validate(p.model, true); err != nil {
		return nil, err
	}

	body := map[string]any{
		"model":  p.model,
		"prompt": params.Prompt,
		"image":  append([]string(nil), params.Images...),
	}
	if params.Quality != "" {
		body["quality"] = params.Quality
	}
	if params.Size != "" {
		body["size"] = params.Size
	}

	baseURL := s.operationBaseURL(opts)
	resp, err := s.postImageRequest(ctx, baseURL, imageEditsEndpoint, body)
	if err != nil {
		return nil, err
	}
	return util.NewSimpleResp(newImageResults(resp)), nil
}

type imageParams struct {
	Prompt  string
	Size    string
	Quality string
	Image   string
	Images  []string
}

func (p *imageParams) Set(name string, val any) xai.Params {
	switch name {
	case ParamPrompt:
		p.Prompt = valueToString(val)
	case ParamSize:
		p.Size = valueToString(val)
	case ParamQuality:
		p.Quality = valueToString(val)
	case ParamImage:
		p.Image = valueToImageInput(val)
	case ParamImages:
		p.Images = valueToImageInputs(val)
	}
	return p
}

func (p *imageParams) validate(model string, edit bool) error {
	if strings.TrimSpace(p.Prompt) == "" {
		return fmt.Errorf("openai: Prompt is required")
	}
	if err := imageQualityRestriction(model).ValidateString(ParamQuality, p.Quality); err != nil {
		return err
	}
	if edit && len(p.Images) == 0 {
		return fmt.Errorf("openai: Images is required")
	}
	return nil
}

func imageQualityRestriction(model string) *xai.Restriction {
	quality := enumImageQuality
	if isGPTImage25Model(xai.Model(model)) {
		quality = enumGPTImage25Quality
	}
	return &xai.Restriction{Limit: quality}
}

func valueToImageInput(val any) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	case xai.Image:
		if u := strings.TrimSpace(v.StgUri()); u != "" {
			return u
		}
		if b := v.Blob(); b != nil {
			return "data:" + string(v.Type()) + ";base64," + b.Base64()
		}
		return ""
	default:
		return valueToString(v)
	}
}

func valueToImageInputs(val any) []string {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []string:
		ret := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(item); s != "" {
				ret = append(ret, s)
			}
		}
		return ret
	case []xai.Image:
		ret := make([]string, 0, len(v))
		for _, item := range v {
			if s := valueToImageInput(item); s != "" {
				ret = append(ret, s)
			}
		}
		return ret
	case []any:
		ret := make([]string, 0, len(v))
		for _, item := range v {
			if s := valueToImageInput(item); s != "" {
				ret = append(ret, s)
			}
		}
		return ret
	default:
		if s := valueToImageInput(v); s != "" {
			return []string{s}
		}
		return nil
	}
}

type imageResponse struct {
	Created      int64  `json:"created"`
	Background   string `json:"background"`
	OutputFormat string `json:"output_format"`
	Size         string `json:"size"`
	Quality      string `json:"quality"`
	Data         []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
	Usage any `json:"usage"`
}

type imageResults struct {
	resp  *imageResponse
	items []*xai.OutputImage
}

func newImageResults(resp *imageResponse) *imageResults {
	ret := &imageResults{resp: resp}
	if resp == nil {
		return ret
	}
	for _, item := range resp.Data {
		switch {
		case strings.TrimSpace(item.URL) != "":
			rawURL := strings.TrimSpace(item.URL)
			ret.items = append(ret.items, &xai.OutputImage{
				Image: &image{
					mime: guessOutputImageType(rawURL, resp.OutputFormat),
					uri:  rawURL,
				},
			})
		case strings.TrimSpace(item.B64JSON) != "":
			format := strings.ToLower(strings.TrimSpace(resp.OutputFormat))
			if format == "" {
				format = "png"
			}
			mime := guessOutputImageType("", format)
			ret.items = append(ret.items, &xai.OutputImage{
				Image: &image{
					mime: mime,
					uri:  "data:" + string(mime) + ";base64," + strings.TrimSpace(item.B64JSON),
				},
			})
		}
	}
	return ret
}

func (p *imageResults) XGo_Attr(name string) any {
	if p.resp == nil {
		return nil
	}
	switch name {
	case "Created":
		return p.resp.Created
	case "Background":
		return p.resp.Background
	case "OutputFormat":
		return p.resp.OutputFormat
	case "Size":
		return p.resp.Size
	case "Quality":
		return p.resp.Quality
	case "Usage":
		return p.resp.Usage
	default:
		return nil
	}
}

func (p *imageResults) Len() int {
	return len(p.items)
}

func (p *imageResults) At(i int) xai.Generated {
	n := len(p.items)
	if i < 0 || i >= n {
		panicIndex("imageResults.At", i, n)
	}
	return p.items[i]
}

func (p *Service) postImageRequest(ctx context.Context, baseURL, endpoint string, body map[string]any) (*imageResponse, error) {
	var ret imageResponse
	if err := p.doOperationJSON(ctx, "POST", baseURL, endpoint, body, &ret); err != nil {
		return nil, err
	}
	return &ret, nil
}

func isGPTImageModel(model xai.Model) bool {
	switch strings.ToLower(strings.TrimSpace(string(model))) {
	case ModelGPTImage2, ModelGPTImage25Flare, ModelGPTImage25Sunburst:
		return true
	default:
		return false
	}
}

func isGPTImage25Model(model xai.Model) bool {
	switch strings.ToLower(strings.TrimSpace(string(model))) {
	case ModelGPTImage25Flare, ModelGPTImage25Sunburst:
		return true
	default:
		return false
	}
}

func guessOutputImageType(rawURL, outputFormat string) xai.ImageType {
	switch strings.ToLower(strings.TrimSpace(outputFormat)) {
	case "jpg", "jpeg":
		return xai.ImageJPEG
	case "gif":
		return xai.ImageGIF
	case "webp":
		return xai.ImageWebP
	case "png":
		return xai.ImagePNG
	}
	if rawURL == "" {
		return xai.ImagePNG
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return xai.ImagePNG
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".jpg", ".jpeg":
		return xai.ImageJPEG
	case ".gif":
		return xai.ImageGIF
	case ".webp":
		return xai.ImageWebP
	default:
		return xai.ImagePNG
	}
}

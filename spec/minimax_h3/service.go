/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 */

package minimax_h3

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"iter"
	"os"

	xai "github.com/goplus/xai/spec"
)

var errGenNotSupported = errors.New("minimax-h3: Gen/GenStream not supported, use Operation for video generation")

// Options implements xai.OptionBuilder.
type Options struct{}

func (p *Options) WithBaseURL(string) xai.OptionBuilder { return p }

// Service implements xai.Service for MiniMax-H3 video generation.
type Service struct {
	backend Backend
	tools   map[string]xai.Tool
}

// NewWithBackend creates a service with a custom backend.
func NewWithBackend(backend Backend) *Service {
	if backend == nil {
		panic("minimax-h3: nil backend")
	}
	return &Service{backend: backend, tools: make(map[string]xai.Tool)}
}

// NewService is an alias for NewWithBackend.
func NewService(backend Backend) *Service { return NewWithBackend(backend) }

// MiniMaxH3Service returns self for provider wrappers.
func (s *Service) MiniMaxH3Service() *Service { return s }

func (*Service) Features() xai.Feature { return xai.FeatureOperation }
func (*Service) Gen(context.Context, xai.ParamBuilder, xai.OptionBuilder) (xai.GenResponse, error) {
	return nil, errGenNotSupported
}
func (*Service) GenStream(context.Context, xai.ParamBuilder, xai.OptionBuilder) iter.Seq2[xai.GenResponse, error] {
	return func(yield func(xai.GenResponse, error) bool) { yield(nil, errGenNotSupported) }
}
func (*Service) Options() xai.OptionBuilder { return &Options{} }
func (*Service) Params() xai.ParamBuilder   { return &noopParamBuilder{} }

type noopParamBuilder struct{}

func (p *noopParamBuilder) System(xai.TextBuilder) xai.ParamBuilder     { return p }
func (p *noopParamBuilder) Messages(...xai.MsgBuilder) xai.ParamBuilder { return p }
func (p *noopParamBuilder) Tools(...xai.ToolBase) xai.ParamBuilder      { return p }
func (p *noopParamBuilder) Model(xai.Model) xai.ParamBuilder            { return p }
func (p *noopParamBuilder) MaxOutputTokens(int64) xai.ParamBuilder      { return p }
func (p *noopParamBuilder) Compact(int64) xai.ParamBuilder              { return p }
func (p *noopParamBuilder) Container(string) xai.ParamBuilder           { return p }
func (p *noopParamBuilder) InferenceGeo(string) xai.ParamBuilder        { return p }
func (p *noopParamBuilder) Temperature(float64) xai.ParamBuilder        { return p }
func (p *noopParamBuilder) TopK(int64) xai.ParamBuilder                 { return p }
func (p *noopParamBuilder) TopP(float64) xai.ParamBuilder               { return p }

type noopImageBuilder struct{}

func (noopImageBuilder) From(xai.ImageType, string, io.Reader) (xai.ImageData, error) {
	return nil, errGenNotSupported
}
func (noopImageBuilder) FromLocal(xai.ImageType, string) (xai.ImageData, error) {
	return nil, errGenNotSupported
}
func (noopImageBuilder) FromBase64(xai.ImageType, string, string) (xai.ImageData, error) {
	return nil, errGenNotSupported
}
func (noopImageBuilder) FromBytes(xai.ImageType, string, []byte) xai.ImageData { return nil }
func (s *Service) Images() xai.ImageBuilder                                    { return noopImageBuilder{} }

type noopDocBuilder struct{}

func (noopDocBuilder) From(xai.DocumentType, string, io.Reader) (xai.DocumentData, error) {
	return nil, errGenNotSupported
}
func (noopDocBuilder) FromLocal(xai.DocumentType, string) (xai.DocumentData, error) {
	return nil, errGenNotSupported
}
func (noopDocBuilder) FromBase64(xai.DocumentType, string, string) (xai.DocumentData, error) {
	return nil, errGenNotSupported
}
func (noopDocBuilder) FromBytes(xai.DocumentType, string, []byte) xai.DocumentData { return nil }
func (noopDocBuilder) PlainText(string) xai.DocumentData                           { return nil }
func (s *Service) Docs() xai.DocumentBuilder                                       { return noopDocBuilder{} }

type noopTextBuilder struct{}

func (noopTextBuilder) Text(string) xai.TextBuilder { return noopTextBuilder{} }
func (*Service) Texts(...string) xai.TextBuilder    { return noopTextBuilder{} }

type noopMsgBuilder struct{}

func (noopMsgBuilder) Text(string) xai.MsgBuilder                      { return noopMsgBuilder{} }
func (noopMsgBuilder) Image(xai.ImageData) xai.MsgBuilder              { return noopMsgBuilder{} }
func (noopMsgBuilder) ImageURL(xai.ImageType, string) xai.MsgBuilder   { return noopMsgBuilder{} }
func (noopMsgBuilder) ImageFile(xai.ImageType, string) xai.MsgBuilder  { return noopMsgBuilder{} }
func (noopMsgBuilder) Doc(xai.DocumentData) xai.MsgBuilder             { return noopMsgBuilder{} }
func (noopMsgBuilder) DocURL(xai.DocumentType, string) xai.MsgBuilder  { return noopMsgBuilder{} }
func (noopMsgBuilder) DocFile(xai.DocumentType, string) xai.MsgBuilder { return noopMsgBuilder{} }
func (noopMsgBuilder) Part(xai.Part) xai.MsgBuilder                    { return noopMsgBuilder{} }
func (noopMsgBuilder) Thinking(xai.Thinking) xai.MsgBuilder            { return noopMsgBuilder{} }
func (noopMsgBuilder) ToolUse(xai.ToolUse) xai.MsgBuilder              { return noopMsgBuilder{} }
func (noopMsgBuilder) ToolResult(xai.ToolResult) xai.MsgBuilder        { return noopMsgBuilder{} }
func (noopMsgBuilder) Compaction(string) xai.MsgBuilder                { return noopMsgBuilder{} }
func (*Service) UserMsg() xai.MsgBuilder                               { return noopMsgBuilder{} }
func (*Service) AssistantMsg() xai.MsgBuilder                          { return noopMsgBuilder{} }

type noopWebSearchTool struct{}

func (noopWebSearchTool) UnderlyingAssignTo(any)                     {}
func (noopWebSearchTool) MaxUses(int64) xai.WebSearchTool            { return noopWebSearchTool{} }
func (noopWebSearchTool) AllowedDomains(...string) xai.WebSearchTool { return noopWebSearchTool{} }
func (noopWebSearchTool) BlockedDomains(...string) xai.WebSearchTool { return noopWebSearchTool{} }
func (*Service) WebSearchTool() xai.WebSearchTool                    { return noopWebSearchTool{} }

type noopTool struct{ name string }

func (noopTool) UnderlyingAssignTo(any)        {}
func (t noopTool) Description(string) xai.Tool { return t }
func (s *Service) ToolDef(name string) xai.Tool {
	if _, ok := s.tools[name]; ok {
		panic("minimax-h3: tool already defined: " + name)
	}
	t := noopTool{name: name}
	s.tools[name] = t
	return t
}
func (s *Service) Tool(name string) xai.Tool { return s.tools[name] }

func (s *Service) ImageFrom(mime xai.ImageType, src io.Reader) (xai.Image, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}
	return s.ImageFromBytes(mime, data), nil
}
func (s *Service) ImageFromLocal(mime xai.ImageType, fileName string) (xai.Image, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	return s.ImageFromBytes(mime, data), nil
}
func (s *Service) ImageFromBase64(mime xai.ImageType, value string) (xai.Image, error) {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	return s.ImageFromBytes(mime, data), nil
}
func (s *Service) ImageFromBytes(mime xai.ImageType, data []byte) xai.Image {
	return newImageFromBytes(mime, data)
}
func (s *Service) ImageFromStgUri(mime xai.ImageType, uri string) xai.Image {
	return newImageFromURI(mime, uri)
}
func (s *Service) VideoFrom(mime xai.VideoType, src io.Reader) (xai.Video, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}
	return s.VideoFromBytes(mime, data), nil
}
func (s *Service) VideoFromLocal(mime xai.VideoType, fileName string) (xai.Video, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	return s.VideoFromBytes(mime, data), nil
}
func (s *Service) VideoFromBase64(mime xai.VideoType, value string) (xai.Video, error) {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	return s.VideoFromBytes(mime, data), nil
}
func (s *Service) VideoFromBytes(mime xai.VideoType, data []byte) xai.Video {
	return newVideoFromBytes(mime, data)
}
func (s *Service) VideoFromStgUri(mime xai.VideoType, uri string) xai.Video {
	return newVideoFromURI(mime, uri)
}
func (*Service) ReferenceImage(xai.Image, int32, xai.ReferenceImageType) (xai.ReferenceImage, xai.Configurable) {
	return nil, nil
}
func (s *Service) GenVideoReferenceImages(...xai.GenVideoReferenceImage) xai.GenVideoReferenceImages {
	return nil
}
func (*Service) GenVideoMask(xai.Image, string) xai.GenVideoMask { return nil }

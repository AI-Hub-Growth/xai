/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 */

package minimax_h3

import (
	xai "github.com/goplus/xai/spec"
	"github.com/goplus/xai/spec/vidu/video"
)

type imageByBytes struct {
	mime xai.ImageType
	data []byte
}

func (p *imageByBytes) Type() xai.ImageType { return p.mime }
func (p *imageByBytes) Blob() xai.BlobData  { return xai.BlobFromRaw(p.data) }
func (p *imageByBytes) StgUri() string      { return "" }

type imageByURI struct {
	mime   xai.ImageType
	stgURI string
}

func (p *imageByURI) Type() xai.ImageType { return p.mime }
func (p *imageByURI) Blob() xai.BlobData  { return nil }
func (p *imageByURI) StgUri() string      { return p.stgURI }

func newImageFromBytes(mime xai.ImageType, data []byte) xai.Image {
	return &imageByBytes{mime: mime, data: data}
}

func newImageFromURI(mime xai.ImageType, uri string) xai.Image {
	return &imageByURI{mime: mime, stgURI: uri}
}

func newVideoFromBytes(mime xai.VideoType, data []byte) xai.Video {
	return video.NewVideoFromBytes(mime, data)
}

func newVideoFromURI(mime xai.VideoType, uri string) xai.Video {
	return video.NewVideoFromURI(mime, uri)
}

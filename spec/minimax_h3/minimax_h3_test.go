package minimax_h3

import (
	"context"
	"strings"
	"testing"

	xai "github.com/goplus/xai/spec"
)

type fakeBackend struct{}

func (fakeBackend) Submit(context.Context, xai.Model, xai.Params) (xai.OperationResponse, error) {
	return &SyncOperationResponse{R: NewOutputVideos([]string{"https://example.com/out.mp4"})}, nil
}
func (fakeBackend) GetTaskStatus(context.Context, string) (xai.OperationResponse, error) {
	return &SyncOperationResponse{R: NewOutputVideos([]string{"https://example.com/out.mp4"})}, nil
}

func TestModelAndSchema(t *testing.T) {
	if !IsVideoModel(ModelMinimaxH3) {
		t.Fatal("expected H3 model to be supported")
	}
	schema := VideoSchemaFor(ModelMinimaxH3)
	if schema == nil {
		t.Fatal("expected video schema")
	}
	if got := len(schema.SupportedModes()); got != 4 {
		t.Fatalf("supported mode count=%d, want 4", got)
	}
	if got := schema.FieldModes(ParamEndImageURL); len(got) != 1 || got[0] != xai.VideoGenModeStartEnd {
		t.Fatalf("end image modes=%v", got)
	}
	if got := GenVideoRestrict(ParamAspectRatio).AllowedValues(); !containsFold(got, "adaptive") {
		t.Fatalf("aspect values=%v", got)
	}
	if IsAspectRatioAllowed(VideoModeTextToVideo, "adaptive") {
		t.Fatal("text-to-video must reject adaptive aspect ratio")
	}
	if !IsAspectRatioAllowed(VideoModeMultiRefToVideo, "adaptive") {
		t.Fatal("multi-reference mode must allow adaptive aspect ratio")
	}
}

func TestValidateModes(t *testing.T) {
	tests := []struct {
		name string
		set  func(*Params)
		want string
	}{
		{name: "text", set: func(p *Params) { p.Set(ParamPrompt, "hello") }, want: VideoModeTextToVideo},
		{name: "image", set: func(p *Params) { p.Set(ParamPrompt, "hello").Set(ParamImageURL, "https://example.com/a.png") }, want: VideoModeImageToVideo},
		{name: "start end", set: func(p *Params) {
			p.Set(ParamPrompt, "hello").Set(ParamVideoMode, VideoModeStartEndToVideo).Set(ParamImageURL, "https://example.com/a.png").Set(ParamEndImageURL, "https://example.com/b.png")
		}, want: VideoModeStartEndToVideo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := NewParams()
			tt.set(params)
			if err := ValidateParams(params); err != nil {
				t.Fatal(err)
			}
			if got := VideoMode(params); got != tt.want {
				t.Fatalf("mode=%q, want %q", got, tt.want)
			}
		})
	}

	audioOnly := NewParams().Set(ParamPrompt, "hello").Set(ParamVideoMode, VideoModeMultiRefToVideo).Set(ParamReferenceAudioURLs, []string{"https://example.com/a.mp3"}).(*Params)
	if err := ValidateParams(audioOnly); err == nil || !strings.Contains(err.Error(), "image or video") {
		t.Fatalf("audio-only validation error=%v", err)
	}

	textAdaptive := NewParams().Set(ParamPrompt, "hello").Set(ParamAspectRatio, "adaptive").(*Params)
	if err := ValidateParams(textAdaptive); err == nil || !strings.Contains(err.Error(), "aspect_ratio") {
		t.Fatalf("text adaptive validation error=%v", err)
	}

	multiAdaptive := NewParams().Set(ParamPrompt, "hello").Set(ParamAspectRatio, "adaptive").Set(ParamReferenceImageURLs, []string{"https://example.com/a.png"}).(*Params)
	if err := ValidateParams(multiAdaptive); err != nil {
		t.Fatalf("multi adaptive validation error=%v", err)
	}

	startEnd := NewParams().Set(ParamPrompt, "hello").Set(ParamImageURL, "https://example.com/a.png").Set(ParamEndImageURL, "https://example.com/b.png").(*Params)
	if err := ValidateParams(startEnd); err != nil {
		t.Fatalf("inferred start/end validation error=%v", err)
	}
	if got := VideoMode(startEnd); got != VideoModeStartEndToVideo {
		t.Fatalf("inferred mode=%q, want %q", got, VideoModeStartEndToVideo)
	}
}

func TestOperationUsesBackend(t *testing.T) {
	svc := NewWithBackend(fakeBackend{})
	op, err := svc.Operation(ModelMinimaxH3, xai.GenVideo)
	if err != nil {
		t.Fatal(err)
	}
	op.Params().Set(ParamPrompt, "hello")
	resp, err := op.Call(context.Background(), svc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Done() || resp.Results().Len() != 1 {
		t.Fatalf("response done=%v results=%v", resp.Done(), resp.Results())
	}
}

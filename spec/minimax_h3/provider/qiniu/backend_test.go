package qiniu

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	xai "github.com/goplus/xai/spec"
	"github.com/goplus/xai/spec/minimax_h3"
)

var quietClientOptions = []ClientOption{
	WithDebugLog(false),
	WithLogger(log.New(io.Discard, "", 0)),
}

func TestBuildVideoRequestRoutes(t *testing.T) {
	tests := []struct {
		name     string
		params   func() *minimax_h3.Params
		endpoint string
		absent   string
	}{
		{
			name: "text",
			params: func() *minimax_h3.Params {
				return minimax_h3.NewParams().Set(minimax_h3.ParamPrompt, "hello").Set(minimax_h3.ParamAspectRatio, "16:9").(*minimax_h3.Params)
			},
			endpoint: EndpointTextToVideo,
		},
		{
			name: "image",
			params: func() *minimax_h3.Params {
				return minimax_h3.NewParams().Set(minimax_h3.ParamPrompt, "hello").Set(minimax_h3.ParamImageURL, "https://example.com/a.png").(*minimax_h3.Params)
			},
			endpoint: EndpointImageToVideo,
			absent:   "aspect_ratio",
		},
		{
			name: "start end",
			params: func() *minimax_h3.Params {
				return minimax_h3.NewParams().Set(minimax_h3.ParamPrompt, "hello").Set(minimax_h3.ParamVideoMode, minimax_h3.VideoModeStartEndToVideo).Set(minimax_h3.ParamImageURL, "https://example.com/a.png").Set(minimax_h3.ParamEndImageURL, "https://example.com/b.png").(*minimax_h3.Params)
			},
			endpoint: EndpointImageToVideo,
		},
		{
			name: "multi reference",
			params: func() *minimax_h3.Params {
				return minimax_h3.NewParams().Set(minimax_h3.ParamPrompt, "hello").Set(minimax_h3.ParamReferenceImageURLs, []string{"https://example.com/a.png"}).Set(minimax_h3.ParamReferenceAudioURLs, []string{"https://example.com/a.mp3"}).(*minimax_h3.Params)
			},
			endpoint: EndpointReferenceToVideo,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := BuildVideoRequest(minimax_h3.ModelMinimaxH3, tt.params())
			if err != nil {
				t.Fatal(err)
			}
			if req.Endpoint != tt.endpoint {
				t.Fatalf("endpoint=%q, want %q", req.Endpoint, tt.endpoint)
			}
			if tt.absent != "" {
				if _, ok := req.Body[tt.absent]; ok {
					t.Fatalf("body unexpectedly contains %q: %#v", tt.absent, req.Body)
				}
			}
			if _, ok := req.Body["generate_audio"]; ok {
				t.Fatal("H3 request must not contain generate_audio")
			}
		})
	}
}

func TestSubmitUsesKeyAuthorization(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"request_id":"request-1"}`))
	}))
	defer server.Close()
	client := NewClient("test-key", append([]ClientOption{WithBaseURL(server.URL)}, quietClientOptions...)...)
	backend := newBackend(client)
	params := minimax_h3.NewParams().Set(minimax_h3.ParamPrompt, "hello")
	if _, err := backend.Submit(context.Background(), xai.Model(minimax_h3.ModelMinimaxH3), params); err != nil {
		t.Fatal(err)
	}
	if gotPath != EndpointTextToVideo || gotAuth != "Key test-key" {
		t.Fatalf("path=%q auth=%q", gotPath, gotAuth)
	}
	if gotBody["prompt"] != "hello" {
		t.Fatalf("body=%#v", gotBody)
	}
}

func TestGetTaskStatusCompletedThenResult(t *testing.T) {
	var statusCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case EndpointStatusPrefix + "request-1/status":
			statusCalls++
			if statusCalls == 1 {
				_, _ = w.Write([]byte(`{"status":"IN_PROGRESS","request_id":"request-1"}`))
			} else {
				_, _ = w.Write([]byte(`{"status":"COMPLETED","detail":null,"request_id":"request-1"}`))
			}
		case EndpointStatusPrefix + "request-1":
			_, _ = w.Write([]byte(`{"video":{"url":"https://example.com/out.mp4"},"usage":{"video_seconds":5}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewClient("test-key", append([]ClientOption{WithBaseURL(server.URL)}, quietClientOptions...)...)
	backend := newBackend(client)
	resp, err := backend.GetTaskStatus(context.Background(), "request-1")
	if err != nil || resp.Done() {
		t.Fatalf("first status resp=%T done=%v err=%v", resp, resp != nil && resp.Done(), err)
	}
	if pending, ok := resp.(*minimax_h3.AsyncOperationResponse); ok {
		pending.SleepDur = 0
	}
	resp, err = resp.Retry(context.Background(), nil)
	if err != nil || !resp.Done() || resp.Results().At(0).(*xai.OutputVideo).URL() != "https://example.com/out.mp4" {
		t.Fatalf("completed resp=%T done=%v err=%v", resp, resp != nil && resp.Done(), err)
	}
}

func TestCompletedDetailIsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"COMPLETED","detail":{"type":"content_policy"}}`))
	}))
	defer server.Close()
	client := NewClient("test-key", append([]ClientOption{WithBaseURL(server.URL)}, quietClientOptions...)...)
	_, err := newBackend(client).GetTaskStatus(context.Background(), "request-1")
	if err == nil || !strings.Contains(err.Error(), "content_policy") {
		t.Fatalf("error=%v", err)
	}
}

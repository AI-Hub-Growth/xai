package qiniu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	xai "github.com/goplus/xai/spec"
	"github.com/goplus/xai/spec/seedance"
)

var testClientOpts = []ClientOption{
	WithDebugLog(false),
	WithLogger(log.New(io.Discard, "", 0)),
}

func TestBackendSubmitAndPollSuccess(t *testing.T) {
	var getCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/v3/contents/generations/tasks"):
			_, _ = w.Write([]byte(`{"id":"qvideo-1"}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/qvideo-1"):
			getCount++
			if getCount < 2 {
				_, _ = w.Write([]byte(`{"status":"running"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"succeeded","content":{"video_url":"https://example.com/out.mp4"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cl := NewClient("test-key", append([]ClientOption{WithBaseURL(srv.URL)}, testClientOpts...)...)
	b := newBackend(cl)
	ctx := context.Background()

	p := seedance.NewParams().
		Set(seedance.ParamPrompt, "hello").
		Set(seedance.ParamRatio, "16:9")
	resp, err := b.Submit(ctx, xai.Model(seedance.ModelDoubaoSeedance20), p)
	if err != nil {
		t.Fatal(err)
	}
	if resp.TaskID() != "qvideo-1" {
		t.Fatalf("task id: %q", resp.TaskID())
	}

	var last xai.OperationResponse = resp
	for !last.Done() {
		last.Sleep()
		last, err = last.Retry(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	if last.Results().Len() != 1 {
		t.Fatalf("len=%d", last.Results().Len())
	}
}

func TestBuildTaskBodyTextToVideo(t *testing.T) {
	p := seedance.NewParams()
	p.Set(seedance.ParamPrompt, "夕阳下的城市街道")
	p.Set("resolution", "720p")
	p.Set(seedance.ParamRatio, "16:9")
	p.Set(seedance.ParamDuration, 5)
	p.Set(seedance.ParamGenerateAudio, true)
	body, err := buildTaskBody(seedance.ModelDoubaoSeedance20, p)
	if err != nil {
		t.Fatal(err)
	}
	if got := body["model"]; got != "bytedance/doubao-seedance-2-0-260128" {
		t.Fatalf("model=%v", got)
	}
	if got := body["resolution"]; got != "720p" {
		t.Fatalf("resolution=%v", got)
	}
	content, ok := body["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("content=%#v", body["content"])
	}
}

func TestBuildTaskBodyKeepsExplicitQiniuModel(t *testing.T) {
	p := seedance.NewParams()
	p.Set(seedance.ParamPrompt, "test")
	body, err := buildTaskBody("bytedance/doubao-seedance-2-0-260128", p)
	if err != nil {
		t.Fatal(err)
	}
	if got := body["model"]; got != "bytedance/doubao-seedance-2-0-260128" {
		t.Fatalf("model=%v", got)
	}
}

func TestBuildTaskBodyKeepsByteplusDreaminaModel(t *testing.T) {
	p := seedance.NewParams()
	p.Set(seedance.ParamPrompt, "test")
	body, err := buildTaskBody(seedance.ModelByteplusDreaminaSeedance20, p)
	if err != nil {
		t.Fatal(err)
	}
	if got := body["model"]; got != seedance.ModelByteplusDreaminaSeedance20 {
		t.Fatalf("model=%v", got)
	}
}

func TestBuildTaskBodyReferenceImageURLsDefaultToReferenceImage(t *testing.T) {
	p := seedance.NewParams()
	p.Set(seedance.ParamPrompt, "从首帧自然过渡到尾帧")
	p.Set(seedance.ParamReferenceImageURLs, []string{"https://example.com/first.png", "https://example.com/last.png"})
	body, err := buildTaskBody(seedance.ModelDoubaoSeedance20, p)
	if err != nil {
		t.Fatal(err)
	}
	content := body["content"].([]any)
	if len(content) != 3 {
		t.Fatalf("content_len=%d", len(content))
	}
	second := content[1].(map[string]any)
	if second["role"] != "reference_image" {
		t.Fatalf("second role=%v", second["role"])
	}
	third := content[2].(map[string]any)
	if third["role"] != "reference_image" {
		t.Fatalf("third role=%v", third["role"])
	}
}

func TestBuildTaskBodyUsesExplicitReferenceImageRoles(t *testing.T) {
	p := seedance.NewParams()
	p.Set(seedance.ParamPrompt, "从首帧自然过渡到尾帧")
	p.Set(seedance.ParamReferenceImages, []map[string]any{
		{"url": "https://example.com/first.png", "role": "first_frame"},
		{"url": "https://example.com/last.png", "role": "last_frame"},
		{"url": "https://example.com/ref.png"},
	})
	body, err := buildTaskBody(seedance.ModelDoubaoSeedance20, p)
	if err != nil {
		t.Fatal(err)
	}
	content := body["content"].([]any)
	if len(content) != 4 {
		t.Fatalf("content_len=%d", len(content))
	}
	if got := content[1].(map[string]any)["role"]; got != "first_frame" {
		t.Fatalf("content[1].role=%v", got)
	}
	if got := content[2].(map[string]any)["role"]; got != "last_frame" {
		t.Fatalf("content[2].role=%v", got)
	}
	if got := content[3].(map[string]any)["role"]; got != "reference_image" {
		t.Fatalf("content[3].role=%v", got)
	}
}

func TestBuildTaskBodyIncludesReferenceVideoAndAudioURLs(t *testing.T) {
	p := seedance.NewParams()
	p.Set(seedance.ParamPrompt, "@视频1 后面添加 @图像1")
	p.Set(seedance.ParamReferenceImageURLs, []string{"https://example.com/image.png"})
	p.Set(seedance.ParamReferenceVideoURLs, []string{"https://example.com/ref.mp4"})
	p.Set(seedance.ParamReferenceAudioURLs, []string{"https://example.com/ref.mp3"})
	body, err := buildTaskBody(seedance.ModelDoubaoSeedance20, p)
	if err != nil {
		t.Fatal(err)
	}
	content := body["content"].([]any)
	if len(content) != 4 {
		t.Fatalf("content_len=%d", len(content))
	}
	video := content[2].(map[string]any)
	if got := video["type"]; got != "video_url" {
		t.Fatalf("video.type=%v", got)
	}
	if got := video["role"]; got != "reference_video" {
		t.Fatalf("video.role=%v", got)
	}
	videoURL, _ := video["video_url"].(map[string]any)
	if got := videoURL["url"]; got != "https://example.com/ref.mp4" {
		t.Fatalf("video.url=%v", got)
	}
	audio := content[3].(map[string]any)
	if got := audio["type"]; got != "audio_url" {
		t.Fatalf("audio.type=%v", got)
	}
	if got := audio["role"]; got != "reference_audio" {
		t.Fatalf("audio.role=%v", got)
	}
	audioURL, _ := audio["audio_url"].(map[string]any)
	if got := audioURL["url"]; got != "https://example.com/ref.mp3" {
		t.Fatalf("audio.url=%v", got)
	}
}

func TestSubmitAutoReviewsReferenceImage(t *testing.T) {
	var createdAsset bool
	var submittedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/assets":
			createdAsset = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if got := body["type"]; got != "image" {
				t.Fatalf("asset type=%v", got)
			}
			if got := body["url"]; got != "https://example.com/actor.png" {
				t.Fatalf("asset url=%v", got)
			}
			if got := body["model"]; got != "bytedance/doubao-seedance-2-0-260128" {
				t.Fatalf("asset model=%v", got)
			}
			_, _ = w.Write([]byte(`{"qassetid":"qasset-1","type":"image","name":"参考图","model":"bytedance/doubao-seedance-2-0-260128","status":"pending","created_at":1,"updated_at":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/assets/qasset-1":
			_, _ = w.Write([]byte(`{"qassetid":"qasset-1","type":"image","name":"参考图","model":"bytedance/doubao-seedance-2-0-260128","status":"approved","created_at":1,"updated_at":2}`))
		case r.Method == http.MethodPost && r.URL.Path == pathCreateTask:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			content := body["content"].([]any)
			image := content[1].(map[string]any)
			imageURL := image["image_url"].(map[string]any)
			submittedURL, _ = imageURL["url"].(string)
			_, _ = w.Write([]byte(`{"id":"qvideo-1"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	t.Setenv("QINIU_ASSETS_BASE_URL", srv.URL)
	cl := NewClient("test-key", append([]ClientOption{WithBaseURL(srv.URL)}, testClientOpts...)...)
	b := newBackend(cl)
	p := seedance.NewParams().
		Set(seedance.ParamPrompt, "真人动起来").
		Set(seedance.ParamReferenceImageURLs, []string{"https://example.com/actor.png"}).
		Set(ParamAssetPollInterval, 1).
		Set(ParamAssetPollAttempts, 1)

	_, err := b.Submit(context.Background(), xai.Model(seedance.ModelDoubaoSeedance20), p)
	if err != nil {
		t.Fatal(err)
	}
	if !createdAsset {
		t.Fatal("expected reference image to be submitted for asset review")
	}
	if submittedURL != "qasset://qasset-1" {
		t.Fatalf("submitted image url=%q", submittedURL)
	}
}

func TestSubmitCanDisableAssetAutoReview(t *testing.T) {
	var assetCalled bool
	var submittedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/assets":
			assetCalled = true
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == pathCreateTask:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			content := body["content"].([]any)
			image := content[1].(map[string]any)
			imageURL := image["image_url"].(map[string]any)
			submittedURL, _ = imageURL["url"].(string)
			_, _ = w.Write([]byte(`{"id":"qvideo-1"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	t.Setenv("QINIU_ASSETS_BASE_URL", srv.URL)
	cl := NewClient("test-key", append([]ClientOption{WithBaseURL(srv.URL)}, testClientOpts...)...)
	b := newBackend(cl)
	disable := false
	p := seedance.NewParams().
		Set(seedance.ParamPrompt, "真人动起来").
		Set(seedance.ParamReferenceImageURLs, []string{"https://example.com/actor.png"}).
		Set(ParamAssetAutoReview, disable)

	_, err := b.Submit(context.Background(), xai.Model(seedance.ModelDoubaoSeedance20), p)
	if err != nil {
		t.Fatal(err)
	}
	if assetCalled {
		t.Fatal("asset review should be disabled")
	}
	if submittedURL != "https://example.com/actor.png" {
		t.Fatalf("submitted image url=%q", submittedURL)
	}
}

func TestSubmitAutoCreateAssetsUsesProviderTemporaryAssets(t *testing.T) {
	var assetCalled bool
	var submittedAutoCreateAssets any
	var submittedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/assets":
			assetCalled = true
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == pathCreateTask:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			submittedAutoCreateAssets = body["auto_create_assets"]
			content := body["content"].([]any)
			image := content[1].(map[string]any)
			imageURL := image["image_url"].(map[string]any)
			submittedURL, _ = imageURL["url"].(string)
			_, _ = w.Write([]byte(`{"id":"qvideo-1"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	t.Setenv("QINIU_ASSETS_BASE_URL", srv.URL)
	cl := NewClient("test-key", append([]ClientOption{WithBaseURL(srv.URL)}, testClientOpts...)...)
	b := newBackend(cl)
	p := seedance.NewParams().
		Set(seedance.ParamPrompt, "真人动起来").
		Set(seedance.ParamReferenceImageURLs, []string{"https://example.com/actor.png"}).
		Set(seedance.ParamAutoCreateAssets, true)

	_, err := b.Submit(context.Background(), xai.Model(seedance.ModelDoubaoSeedance20), p)
	if err != nil {
		t.Fatal(err)
	}
	if assetCalled {
		t.Fatal("provider temporary assets should bypass manual asset review")
	}
	if submittedAutoCreateAssets != true {
		t.Fatalf("auto_create_assets=%v, want true", submittedAutoCreateAssets)
	}
	if submittedURL != "https://example.com/actor.png" {
		t.Fatalf("submitted image url=%q", submittedURL)
	}
}

func TestSubmitByteplusDreaminaSkipsAssetReview(t *testing.T) {
	var assetCalled bool
	var submittedModel string
	var submittedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/assets":
			assetCalled = true
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == pathCreateTask:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			submittedModel, _ = body["model"].(string)
			content := body["content"].([]any)
			image := content[1].(map[string]any)
			imageURL := image["image_url"].(map[string]any)
			submittedURL, _ = imageURL["url"].(string)
			_, _ = w.Write([]byte(`{"id":"qvideo-1"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("QINIU_ASSETS_BASE_URL", srv.URL)
	cl := NewClient("test-key", append([]ClientOption{WithBaseURL(srv.URL)}, testClientOpts...)...)
	b := newBackend(cl)
	p := seedance.NewParams().
		Set(seedance.ParamPrompt, "真人动起来").
		Set(seedance.ParamReferenceImageURLs, []string{"https://example.com/actor.png"})

	_, err := b.Submit(context.Background(), xai.Model(seedance.ModelByteplusDreaminaSeedance20), p)
	if err != nil {
		t.Fatal(err)
	}
	if assetCalled {
		t.Fatal("byteplus dreamina model should not use asset review")
	}
	if submittedModel != seedance.ModelByteplusDreaminaSeedance20 {
		t.Fatalf("submitted model=%q", submittedModel)
	}
	if submittedURL != "https://example.com/actor.png" {
		t.Fatalf("submitted image url=%q", submittedURL)
	}
}

func TestSubmitRetriesRetryableAssetReviewFailure(t *testing.T) {
	var createCount int
	var submittedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/assets":
			createCount++
			_, _ = w.Write([]byte(fmt.Sprintf(`{"qassetid":"qasset-%d","type":"image","name":"参考图","model":"bytedance/doubao-seedance-2-0-260128","status":"pending","created_at":1,"updated_at":1}`, createCount)))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/assets/qasset-1":
			_, _ = w.Write([]byte(`{"qassetid":"qasset-1","type":"image","name":"参考图","model":"bytedance/doubao-seedance-2-0-260128","status":"failed","fail_reason":"[INFRA ASSET API ERROR] Asset API request failed","created_at":1,"updated_at":2}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/assets/qasset-2":
			_, _ = w.Write([]byte(`{"qassetid":"qasset-2","type":"image","name":"参考图","model":"bytedance/doubao-seedance-2-0-260128","status":"approved","created_at":1,"updated_at":2}`))
		case r.Method == http.MethodPost && r.URL.Path == pathCreateTask:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			content := body["content"].([]any)
			image := content[1].(map[string]any)
			imageURL := image["image_url"].(map[string]any)
			submittedURL, _ = imageURL["url"].(string)
			_, _ = w.Write([]byte(`{"id":"qvideo-1"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("QINIU_ASSETS_BASE_URL", srv.URL)
	cl := NewClient("test-key", append([]ClientOption{WithBaseURL(srv.URL)}, testClientOpts...)...)
	b := newBackend(cl)
	p := seedance.NewParams().
		Set(seedance.ParamPrompt, "真人动起来").
		Set(seedance.ParamReferenceImageURLs, []string{"https://example.com/actor.png"}).
		Set(ParamAssetPollInterval, 1).
		Set(ParamAssetPollAttempts, 1).
		Set(ParamAssetReviewRetries, 1)

	_, err := b.Submit(context.Background(), xai.Model(seedance.ModelDoubaoSeedance20), p)
	if err != nil {
		t.Fatal(err)
	}
	if createCount != 2 {
		t.Fatalf("createCount=%d", createCount)
	}
	if submittedURL != "qasset://qasset-2" {
		t.Fatalf("submitted image url=%q", submittedURL)
	}
}

func TestSubmitDoesNotRetryPolicyAssetReviewFailure(t *testing.T) {
	var createCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/assets":
			createCount++
			_, _ = w.Write([]byte(`{"qassetid":"qasset-1","type":"image","name":"参考图","model":"bytedance/doubao-seedance-2-0-260128","status":"pending","created_at":1,"updated_at":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/assets/qasset-1":
			_, _ = w.Write([]byte(`{"qassetid":"qasset-1","type":"image","name":"参考图","model":"bytedance/doubao-seedance-2-0-260128","status":"failed","fail_reason":"content policy rejected","created_at":1,"updated_at":2}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("QINIU_ASSETS_BASE_URL", srv.URL)
	cl := NewClient("test-key", append([]ClientOption{WithBaseURL(srv.URL)}, testClientOpts...)...)
	b := newBackend(cl)
	p := seedance.NewParams().
		Set(seedance.ParamPrompt, "真人动起来").
		Set(seedance.ParamReferenceImageURLs, []string{"https://example.com/actor.png"}).
		Set(ParamAssetPollInterval, 1).
		Set(ParamAssetPollAttempts, 1).
		Set(ParamAssetReviewRetries, 3)

	_, err := b.Submit(context.Background(), xai.Model(seedance.ModelDoubaoSeedance20), p)
	if err == nil {
		t.Fatal("expected policy failure")
	}
	if createCount != 1 {
		t.Fatalf("createCount=%d", createCount)
	}
}

func TestSubmitOmniReferenceTaskTypeHTTPBody(t *testing.T) {
	for _, taskType := range []string{"edit", "reference", ""} {
		t.Run("mode="+taskType, func(t *testing.T) {
			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != pathCreateTask {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				_, _ = w.Write([]byte(`{"id":"qvideo-edit"}`))
			}))
			defer srv.Close()
			svc := NewService("test-key", append([]ClientOption{WithBaseURL(srv.URL)}, testClientOpts...)...)
			op, err := svc.Operation(xai.Model("doubao-seedance-2-5-260628"), xai.GenVideo)
			if err != nil {
				t.Fatal(err)
			}
			op.Params().Set(seedance.ParamPrompt, "Replace the screen content").
				Set(seedance.ParamReferenceVideoURLs, []string{"qasset://approved-video"}).
				Set(seedance.ParamRatio, "adaptive").
				Set(seedance.ParamDuration, -1).
				Set(seedance.ParamOmniReferenceTaskType, taskType)
			if _, err := op.Call(context.Background(), svc, svc.Options()); err != nil {
				t.Fatal(err)
			}
			if taskType == "" {
				if _, exists := body["omni_reference_task_type"]; exists {
					t.Fatal("unspecified task type must be omitted")
				}
			} else if body["omni_reference_task_type"] != taskType {
				t.Fatalf("task type = %v, want %s", body["omni_reference_task_type"], taskType)
			}
			if body["duration"] != float64(-1) || body["ratio"] != "adaptive" {
				t.Fatalf("edit controls lost: %#v", body)
			}
			content := body["content"].([]any)
			video := content[1].(map[string]any)
			if video["role"] != "reference_video" || video["video_url"].(map[string]any)["url"] != "qasset://approved-video" {
				t.Fatalf("target video changed: %#v", video)
			}
		})
	}
}

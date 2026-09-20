package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	xai "github.com/goplus/xai/spec"
)

func TestGPTImageActionsAndOperation(t *testing.T) {
	svc := &Service{}

	for _, model := range []string{ModelGPTImage2, ModelGPTImage25Flare, ModelGPTImage25Sunburst} {
		actions := svc.Actions(xai.Model(model))
		if len(actions) != 2 || actions[0] != xai.GenImage || actions[1] != xai.EditImage {
			t.Fatalf("unexpected actions for %s: %v", model, actions)
		}
	}

	if got := svc.Actions("gpt-4o"); len(got) != 0 {
		t.Fatalf("expected no actions for gpt-4o, got: %v", got)
	}

	if _, err := svc.Operation(ModelGPTImage2, xai.GenImage); err != nil {
		t.Fatalf("operation gptimage/gen_image failed: %v", err)
	}
	if _, err := svc.Operation(ModelGPTImage2, xai.EditImage); err != nil {
		t.Fatalf("operation gptimage/edit_image failed: %v", err)
	}
	if _, err := svc.Operation(ModelGPTImage2, xai.GenVideo); !errors.Is(err, xai.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for gptimage/gen_video, got: %v", err)
	}
}

func TestGPTImage25GenerateUsesImagesAPIAndExtendedQuality(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body failed: %v", err)
		}
		if body["model"] != ModelGPTImage25Flare || body["quality"] != "max" {
			t.Fatalf("unexpected request body: %#v", body)
		}
		_, _ = w.Write([]byte(`{"created":1,"data":[{"b64_json":"aGVsbG8="}]}`))
	}))
	defer ts.Close()

	svc := &Service{baseURL: ts.URL + "/v1/", apiKey: "token-1", httpClient: ts.Client()}
	op, err := svc.Operation(ModelGPTImage25Flare, xai.GenImage)
	if err != nil {
		t.Fatalf("Operation failed: %v", err)
	}
	if err := op.InputSchema().Restrict(ParamQuality).ValidateString(ParamQuality, "max"); err != nil {
		t.Fatalf("2.5 schema rejected max quality: %v", err)
	}
	op.Params().Set("Prompt", "hello").Set("Quality", "max")
	if _, err := op.Call(context.Background(), svc, nil); err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	legacy, err := svc.Operation(ModelGPTImage2, xai.GenImage)
	if err != nil {
		t.Fatalf("legacy Operation failed: %v", err)
	}
	legacy.Params().Set("Prompt", "hello").Set("Quality", "max")
	if _, err := legacy.Call(context.Background(), svc, nil); err == nil || !strings.Contains(err.Error(), "Quality") {
		t.Fatalf("expected legacy quality validation error, got: %v", err)
	}
}

func TestGPTImageGenerate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-1" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body failed: %v", err)
		}
		if body["model"] != ModelGPTImage2 {
			t.Fatalf("unexpected model: %#v", body["model"])
		}
		if body["prompt"] != "可爱的少女，动漫" {
			t.Fatalf("unexpected prompt: %#v", body["prompt"])
		}
		if body["quality"] != "high" {
			t.Fatalf("unexpected quality: %#v", body["quality"])
		}
		if body["image"] != "https://example.com/source.png" {
			t.Fatalf("unexpected image: %#v", body["image"])
		}
		_, _ = w.Write([]byte(`{
			"created": 1,
			"output_format": "png",
			"quality": "high",
			"data": [{"b64_json":"aGVsbG8="}]
		}`))
	}))
	defer ts.Close()

	svc := &Service{
		baseURL:    ts.URL + "/v1/",
		apiKey:     "token-1",
		httpClient: ts.Client(),
	}

	op, err := svc.Operation(ModelGPTImage2, xai.GenImage)
	if err != nil {
		t.Fatalf("Operation failed: %v", err)
	}
	op.Params().
		Set("Prompt", "可爱的少女，动漫").
		Set("Quality", "high").
		Set("Image", "https://example.com/source.png")

	resp, err := op.Call(context.Background(), svc, nil)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if !resp.Done() {
		t.Fatal("expected sync response")
	}
	results := resp.Results()
	if results.Len() != 1 {
		t.Fatalf("unexpected results len: %d", results.Len())
	}
	imgOut := results.At(0).(*xai.OutputImage)
	if got := imgOut.URL(); got != "data:image/png;base64,aGVsbG8=" {
		t.Fatalf("unexpected image url: %s", got)
	}
	if got := imgOut.Image.Type(); got != xai.ImagePNG {
		t.Fatalf("unexpected image type: %s", got)
	}
}

func TestGPTImageEdit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/edits" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body failed: %v", err)
		}
		if body["prompt"] != "图片中增加一个人" {
			t.Fatalf("unexpected prompt: %#v", body["prompt"])
		}
		if body["quality"] != "low" {
			t.Fatalf("unexpected quality: %#v", body["quality"])
		}
		images, ok := body["image"].([]any)
		if !ok || len(images) != 2 {
			t.Fatalf("unexpected image list: %#v", body["image"])
		}
		if images[0] != "https://example.com/one.jpg" {
			t.Fatalf("unexpected image[0]: %#v", images[0])
		}
		if !strings.HasPrefix(images[1].(string), "data:image/png;base64,") {
			t.Fatalf("unexpected image[1]: %#v", images[1])
		}
		_, _ = w.Write([]byte(`{
			"created": 2,
			"data": [{"url":"https://example.com/edited.webp"}]
		}`))
	}))
	defer ts.Close()

	svc := &Service{
		baseURL:    ts.URL + "/v1/",
		apiKey:     "token-1",
		httpClient: ts.Client(),
	}

	op, err := svc.Operation(ModelGPTImage2, xai.EditImage)
	if err != nil {
		t.Fatalf("Operation failed: %v", err)
	}
	op.Params().
		Set("Prompt", "图片中增加一个人").
		Set("Quality", "low").
		Set("Images", []any{
			"https://example.com/one.jpg",
			svc.ImageFromBytes(xai.ImagePNG, []byte("hello")),
		})

	resp, err := op.Call(context.Background(), svc, nil)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if !resp.Done() {
		t.Fatal("expected sync response")
	}
	results := resp.Results()
	if results.Len() != 1 {
		t.Fatalf("unexpected results len: %d", results.Len())
	}
	imgOut := results.At(0).(*xai.OutputImage)
	if got := imgOut.URL(); got != "https://example.com/edited.webp" {
		t.Fatalf("unexpected image url: %s", got)
	}
	if got := imgOut.Image.Type(); got != xai.ImageWebP {
		t.Fatalf("unexpected image type: %s", got)
	}
}

func TestGPTImageValidationAndBaseURLOverride(t *testing.T) {
	svc := &Service{
		baseURL: defaultAPIBaseURL,
		apiKey:  "token-1",
	}

	testErr := func(t *testing.T, action xai.Action, want string, setup func(xai.Operation)) {
		t.Helper()
		op, err := svc.Operation(ModelGPTImage2, action)
		if err != nil {
			t.Fatalf("Operation failed: %v", err)
		}
		setup(op)
		_, err = op.Call(context.Background(), svc, nil)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error containing %q, got: %v", want, err)
		}
	}

	testErr(t, xai.GenImage, "Prompt is required", func(op xai.Operation) {})
	testErr(t, xai.EditImage, "Images is required", func(op xai.Operation) {
		op.Params().Set("Prompt", "hello")
	})
	testErr(t, xai.GenImage, "Quality", func(op xai.Operation) {
		op.Params().Set("Prompt", "hello").Set("Quality", "ultra")
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/alt/images/generations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"created":1,"output_format":"jpeg","data":[{"url":"https://example.com/out.jpg"}]}`))
	}))
	defer ts.Close()

	op, err := svc.Operation(ModelGPTImage2, xai.GenImage)
	if err != nil {
		t.Fatalf("Operation failed: %v", err)
	}
	op.Params().Set("Prompt", "hello")

	resp, err := op.Call(context.Background(), svc, svc.Options().WithBaseURL(ts.URL+"/alt/"))
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	imgOut := resp.Results().At(0).(*xai.OutputImage)
	if got := imgOut.Image.Type(); got != xai.ImageJPEG {
		t.Fatalf("unexpected image type: %s", got)
	}
}

func TestSoraActionsAndOperation(t *testing.T) {
	svc := &Service{}

	actions := svc.Actions("sora-2")
	if len(actions) != 1 || actions[0] != xai.GenVideo {
		t.Fatalf("unexpected actions for sora-2: %v", actions)
	}

	if got := svc.Actions("gpt-4o"); len(got) != 0 {
		t.Fatalf("expected no actions for gpt-4o, got: %v", got)
	}

	if _, err := svc.Operation("sora-2", xai.GenVideo); err != nil {
		t.Fatalf("operation sora/gen_video failed: %v", err)
	}

	if _, err := svc.Operation("gpt-4o", xai.GenVideo); !errors.Is(err, xai.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for gpt-4o/gen_video, got: %v", err)
	}
}

func TestSoraVideoCallAndRetry(t *testing.T) {
	const (
		taskID   = "qvideo-user123-1766453713089395279"
		videoURL = "https://aitoken-video.qnaigc.com/user123/qvideo-user123-1766453713089395279/1.mp4"
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos":
			if got := r.Header.Get("Authorization"); got != "Bearer token-1" {
				t.Fatalf("unexpected auth header: %s", got)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body failed: %v", err)
			}
			if body["model"] != "sora-2" {
				t.Fatalf("unexpected model: %#v", body["model"])
			}
			if body["prompt"] != "A cute cat running in a garden" {
				t.Fatalf("unexpected prompt: %#v", body["prompt"])
			}
			if body["input_reference"] != "https://example.com/cat.jpg" {
				t.Fatalf("unexpected input_reference: %#v", body["input_reference"])
			}
			if body["seconds"] != "4" {
				t.Fatalf("unexpected seconds: %#v", body["seconds"])
			}
			if body["size"] != "1280x720" {
				t.Fatalf("unexpected size: %#v", body["size"])
			}
			_, _ = w.Write([]byte(`{
				"id":"` + taskID + `",
				"object":"video",
				"model":"sora-2",
				"status":"queued",
				"created_at":1766453713,
				"updated_at":1766453713,
				"seconds":"4",
				"size":"1280x720"
			}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/"+taskID:
			_, _ = w.Write([]byte(`{
				"id":"` + taskID + `",
				"object":"video",
				"model":"sora-2",
				"status":"completed",
				"created_at":1766453713,
				"updated_at":1766453836,
				"completed_at":1766453836,
				"seconds":"4",
				"size":"1280x720",
				"task_result":{
					"videos":[
						{
							"id":"` + taskID + `-1",
							"url":"` + videoURL + `",
							"duration":"4"
						}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	svc := &Service{
		baseURL:    ts.URL + "/v1/",
		apiKey:     "token-1",
		httpClient: ts.Client(),
	}

	op, err := svc.Operation("sora-2", xai.GenVideo)
	if err != nil {
		t.Fatalf("Operation failed: %v", err)
	}
	op.Params().
		Set("Prompt", "A cute cat running in a garden").
		Set("InputReference", "https://example.com/cat.jpg").
		Set("Seconds", int32(4)).
		Set("Size", "1280x720")

	resp, err := op.Call(context.Background(), svc, nil)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.Done() {
		t.Fatal("expected queued response")
	}
	if got := resp.TaskID(); got != taskID {
		t.Fatalf("unexpected task id: %q", got)
	}

	resp, err = resp.Retry(context.Background(), svc)
	if err != nil {
		t.Fatalf("Retry failed: %v", err)
	}
	if !resp.Done() {
		t.Fatal("expected completed response")
	}
	if got := resp.TaskID(); got != taskID {
		t.Fatalf("unexpected task id after retry: %q", got)
	}

	results := resp.Results()
	if got, want := results.Len(), 1; got != want {
		t.Fatalf("results len got=%d want=%d", got, want)
	}
	out := results.At(0).(*xai.OutputVideo)
	if got := out.URL(); got != videoURL {
		t.Fatalf("unexpected output url: %s", got)
	}
	if got := out.Video.Type(); got != xai.VideoMP4 {
		t.Fatalf("unexpected output type: %s", got)
	}
}

func TestSoraVideoRemix(t *testing.T) {
	const (
		sourceID = "qvideo-user123-source"
		taskID   = "qvideo-user123-remix"
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/videos/"+sourceID+"/remix" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body failed: %v", err)
		}
		if body["prompt"] != "Change scene to neon cyberpunk night" {
			t.Fatalf("unexpected prompt: %#v", body["prompt"])
		}
		if _, ok := body["seconds"]; ok {
			t.Fatalf("unexpected seconds in remix body: %#v", body["seconds"])
		}
		if _, ok := body["size"]; ok {
			t.Fatalf("unexpected size in remix body: %#v", body["size"])
		}
		_, _ = w.Write([]byte(`{
			"id":"` + taskID + `",
			"object":"video",
			"model":"sora-2",
			"status":"queued",
			"remixed_from_video_id":"` + sourceID + `"
		}`))
	}))
	defer ts.Close()

	svc := &Service{
		baseURL:    ts.URL + "/v1/",
		apiKey:     "token-1",
		httpClient: ts.Client(),
	}

	op, err := svc.Operation("sora-2", xai.GenVideo)
	if err != nil {
		t.Fatalf("Operation failed: %v", err)
	}
	op.Params().
		Set("Prompt", "Change scene to neon cyberpunk night").
		Set("RemixFromVideoID", sourceID)

	resp, err := op.Call(context.Background(), svc, nil)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.Done() {
		t.Fatal("expected queued response")
	}
	if got := resp.TaskID(); got != taskID {
		t.Fatalf("unexpected task id: %q", got)
	}
}

func TestSoraVideoValidation(t *testing.T) {
	svc := &Service{
		baseURL: defaultAPIBaseURL,
		apiKey:  "token-1",
	}

	testErr := func(t *testing.T, want string, setup func(xai.Operation)) {
		t.Helper()
		op, err := svc.Operation("sora-2", xai.GenVideo)
		if err != nil {
			t.Fatalf("Operation failed: %v", err)
		}
		setup(op)
		_, err = op.Call(context.Background(), svc, nil)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error containing %q, got: %v", want, err)
		}
	}

	testErr(t, "Prompt is required", func(op xai.Operation) {})
	testErr(t, "Seconds", func(op xai.Operation) {
		op.Params().Set("Prompt", "hello").Set("Seconds", "5")
	})
	testErr(t, "Size is not supported in remix mode", func(op xai.Operation) {
		op.Params().Set("Prompt", "hello").Set("RemixFromVideoID", "qvideo-src").Set("Size", "1280x720")
	})
	testErr(t, "Size", func(op xai.Operation) {
		op.Params().Set("Prompt", "hello").Set("Size", "1024x1792") // sora-2 does not support 1024x1792
	})
	testErr(t, "2500", func(op xai.Operation) {
		op.Params().Set("Prompt", strings.Repeat("x", 2501))
	})
	testErr(t, "RemixFromVideoID", func(op xai.Operation) {
		op.Params().Set("Prompt", "remix").Set("RemixFromVideoID", "/") // invalid, normalizes to empty
	})

	// sora-2-pro accepts 1024x1792 (validation passes)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/videos" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["model"] != "sora-2-pro" || body["size"] != "1024x1792" {
				t.Fatalf("unexpected body: %v", body)
			}
			_, _ = w.Write([]byte(`{"id":"qvideo-pro","object":"video","model":"sora-2-pro","status":"queued"}`))
		}
	}))
	defer ts.Close()
	svcPro := &Service{baseURL: ts.URL + "/v1/", apiKey: "token-1", httpClient: ts.Client()}
	opPro, _ := svcPro.Operation("sora-2-pro", xai.GenVideo)
	opPro.Params().Set("Prompt", "test").Set("Size", "1024x1792")
	if _, err := opPro.Call(context.Background(), svcPro, nil); err != nil {
		t.Fatalf("sora-2-pro with 1024x1792 should pass, got: %v", err)
	}
}

func TestSoraVideoFailedResponseError(t *testing.T) {
	const taskID = "qvideo-user123-failed"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos":
			_, _ = w.Write([]byte(`{
				"id":"` + taskID + `",
				"object":"video",
				"model":"sora-2",
				"status":"queued"
			}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/"+taskID:
			_, _ = w.Write([]byte(`{
				"id":"` + taskID + `",
				"object":"video",
				"model":"sora-2",
				"status":"failed",
				"error":{
					"code":"moderation_blocked",
					"message":"Your request was blocked by moderation."
				}
			}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	svc := &Service{
		baseURL:    ts.URL + "/v1/",
		apiKey:     "token-1",
		httpClient: ts.Client(),
	}
	op, _ := svc.Operation("sora-2", xai.GenVideo)
	op.Params().Set("Prompt", "unsafe prompt")

	resp, err := op.Call(context.Background(), svc, nil)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	resp, err = resp.Retry(context.Background(), svc)
	if err != nil {
		t.Fatalf("Retry failed: %v", err)
	}
	if !resp.Done() {
		t.Fatal("expected failed response to be done")
	}
	if got := resp.Results().Len(); got != 0 {
		t.Fatalf("expected no output videos, got %d", got)
	}

	errResp, ok := resp.(xai.OperationResponseWithError)
	if !ok {
		t.Fatalf("expected OperationResponseWithError, got %T", resp)
	}
	gotErr := errResp.GetError()
	if gotErr == nil || !strings.Contains(gotErr.Error(), "moderation_blocked") {
		t.Fatalf("unexpected operation error: %v", gotErr)
	}
}

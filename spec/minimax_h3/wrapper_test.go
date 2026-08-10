package minimax_h3

import (
	"context"
	"testing"

	xai "github.com/goplus/xai/spec"
)

func TestRegisterAndGetTask(t *testing.T) {
	svc := NewWithBackend(fakeBackend{})
	Register(svc)
	registered, err := xai.New(context.Background(), Scheme+":")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xai.GetTask(context.Background(), registered, ModelMinimaxH3, xai.GenVideo, "request-1"); err != nil {
		t.Fatal(err)
	}
}

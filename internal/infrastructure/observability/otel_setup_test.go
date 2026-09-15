package observability

import (
	"context"
	"testing"
)

func TestSetupTracingWithEndpoint(t *testing.T) {
	shutdown, err := SetupTracing(context.Background(), "test", "localhost:4317")
	if err != nil {
		t.Fatalf("SetupTracing: %v", err)
	}
	_ = shutdown(context.Background())
}

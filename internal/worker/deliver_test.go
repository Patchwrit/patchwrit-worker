package worker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Patchwrit/patchwrit-worker/internal/contract"
)

func TestDeliverPostsResultWithBearerToken(t *testing.T) {
	var gotAuth, gotContentType string
	var got contract.Result

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("callback body is not a contract result: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	res := BuildResult("job-9", time.Now(), func() contract.Environment {
		return contract.Environment{GoVersion: "go1.26.2"}
	})

	if err := Deliver(context.Background(), server.Client(), server.URL, "job-token", res); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if gotAuth != "Bearer job-token" {
		t.Fatalf("Authorization = %q, want Bearer job-token", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q", gotContentType)
	}
	if got.JobID != "job-9" || got.Status != contract.StatusCompleted {
		t.Fatalf("delivered result = %+v", got)
	}
}

func TestDeliverRejectsNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	res := BuildResult("job-9", time.Now(), SystemDetect)
	if err := Deliver(context.Background(), server.Client(), server.URL, "bad", res); err == nil {
		t.Fatal("Deliver must error on non-2xx so Phoenix can classify the crash")
	}
}

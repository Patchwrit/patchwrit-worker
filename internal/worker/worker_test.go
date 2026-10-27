package worker

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Patchwrit/patchwrit-worker/internal/contract"
)

func TestBuildResultSerializesContractShape(t *testing.T) {
	git := "git version 2.47.0"
	detect := func() contract.Environment {
		return contract.Environment{GoVersion: "go1.26.2", GitVersion: &git}
	}

	result := BuildResult("job-123", time.Now(), detect)

	if result.SchemaVersion != contract.SchemaVersion {
		t.Fatalf("schema_version = %d, want %d", result.SchemaVersion, contract.SchemaVersion)
	}
	if result.JobID != "job-123" || result.Status != contract.StatusCompleted {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Environment.GitVersion == nil || *result.Environment.GitVersion != git {
		t.Fatalf("git_version = %v, want %q", result.Environment.GitVersion, git)
	}
	if result.Environment.JavaVersion != nil || result.Environment.MavenVersion != nil {
		t.Fatal("missing tools must serialize as null, not empty strings")
	}

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"errors":[]`) {
		t.Fatalf("errors must serialize as [], got %s", raw)
	}
	if !strings.Contains(string(raw), `"java_version":null`) {
		t.Fatalf("missing tools must serialize as null, got %s", raw)
	}
}

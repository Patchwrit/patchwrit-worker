package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildResultSerializesContractShape(t *testing.T) {
	runner := func(name string, args ...string) (string, bool) {
		if name == "git" {
			return "git version 2.47.0", true
		}
		return "", false
	}

	result := buildResult("job-123", time.Unix(0, 0), runner)

	if result.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", result.SchemaVersion)
	}
	if result.JobID != "job-123" {
		t.Fatalf("job_id = %q, want job-123", result.JobID)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if result.Environment.GoVersion == "" {
		t.Fatal("go_version must be reported")
	}
	if result.Environment.GitVersion == nil || *result.Environment.GitVersion != "git version 2.47.0" {
		t.Fatalf("git_version = %v, want detected version", result.Environment.GitVersion)
	}
	if result.Environment.JavaVersion != nil || result.Environment.MavenVersion != nil {
		t.Fatal("missing tools must serialize as null, not empty strings")
	}
	if result.Errors == nil || len(result.Errors) != 0 {
		t.Fatalf("errors = %v, want empty array", result.Errors)
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

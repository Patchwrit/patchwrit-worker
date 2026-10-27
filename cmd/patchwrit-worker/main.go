// Command patchwrit-worker is the ephemeral Patchwrit execution worker.
//
// W2 smoke scope: report the contract result JSON (schema_version 1) with
// detected toolchain versions and exit 0. Cloning, Maven, OSV, and the
// Phoenix callback arrive in later milestones; the result is printed to
// stdout so it can be smoke-tested locally and in a container.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// workerVersion is stamped at build time via -ldflags "-X main.workerVersion=...".
var workerVersion = "0.1.0"

type environment struct {
	GoVersion    string  `json:"go_version"`
	GitVersion   *string `json:"git_version"`
	JavaVersion  *string `json:"java_version"`
	MavenVersion *string `json:"maven_version"`
}

type jobError struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

type result struct {
	SchemaVersion int              `json:"schema_version"`
	JobID         string           `json:"job_id"`
	Status        string           `json:"status"`
	WorkerVersion string           `json:"worker_version"`
	Environment   environment      `json:"environment"`
	TimingsMs     map[string]int64 `json:"timings_ms"`
	Errors        []jobError       `json:"errors"`
}

// versionRunner reports the first line of a tool's version output.
// It returns false when the tool is missing or fails to run.
type versionRunner func(name string, args ...string) (string, bool)

func execVersion(name string, args ...string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}

	// CombinedOutput: `java -version` writes to stderr, not stdout.
	out, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return "", false
	}

	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	if line == "" {
		return "", false
	}
	return line, true
}

func versionPtr(runner versionRunner, name string, args ...string) *string {
	if version, ok := runner(name, args...); ok {
		return &version
	}
	return nil
}

func buildResult(jobID string, start time.Time, runner versionRunner) result {
	return result{
		SchemaVersion: 1,
		JobID:         jobID,
		Status:        "completed",
		WorkerVersion: workerVersion,
		Environment: environment{
			GoVersion:    runtime.Version(),
			GitVersion:   versionPtr(runner, "git", "--version"),
			JavaVersion:  versionPtr(runner, "java", "-version"),
			MavenVersion: versionPtr(runner, "mvn", "-version"),
		},
		TimingsMs: map[string]int64{
			"execution": time.Since(start).Milliseconds(),
		},
		Errors: []jobError{},
	}
}

func main() {
	start := time.Now()

	jobID := os.Getenv("PATCHWRIT_JOB_ID")
	if jobID == "" {
		jobID = "local-smoke"
	}

	res := buildResult(jobID, start, execVersion)

	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		fmt.Fprintf(os.Stderr, "patchwrit-worker: encode result: %v\n", err)
		os.Exit(1)
	}
}

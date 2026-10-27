// Package worker assembles the contract result for a job.
package worker

import (
	"time"

	"github.com/Patchwrit/patchwrit-worker/internal/contract"
	"github.com/Patchwrit/patchwrit-worker/internal/toolchain"
)

// Version is the worker release reported in every result.
// Stamped at build time via -ldflags "-X .../internal/worker.Version=...".
var Version = "0.1.0"

// DetectFunc reports the toolchain environment. Injectable for tests.
type DetectFunc func() contract.Environment

// SystemDetect is the production DetectFunc.
func SystemDetect() contract.Environment {
	return toolchain.Detect(toolchain.ExecRunner)
}

// BuildResult builds the smoke result: environment detection only.
// Cloning, Maven, OSV, and callback delivery extend this in later
// milestones without changing the result shape.
func BuildResult(jobID string, start time.Time, detect DetectFunc) contract.Result {
	return contract.Result{
		SchemaVersion: contract.SchemaVersion,
		JobID:         jobID,
		Status:        contract.StatusCompleted,
		WorkerVersion: Version,
		Environment:   detect(),
		TimingsMs: map[string]int64{
			"execution": time.Since(start).Milliseconds(),
		},
		Errors: []contract.Error{},
	}
}

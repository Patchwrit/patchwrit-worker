// Package worker assembles the contract result for a job.
package worker

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/Patchwrit/patchwrit-worker/internal/checkout"
	"github.com/Patchwrit/patchwrit-worker/internal/contract"
	"github.com/Patchwrit/patchwrit-worker/internal/maven"
	"github.com/Patchwrit/patchwrit-worker/internal/observe"
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

// Input is one job as delivered through PATCHWRIT_* environment variables.
type Input struct {
	JobID         string
	RepositoryURL string
	PinnedSHA     string
}

// Execute builds the result and, when a repository is specified, clones
// it at the pinned SHA and inventories its POM structure. A failed clone
// is a reported failure (status failed, categorized error), not a crash.
func Execute(ctx context.Context, in Input, start time.Time, detect DetectFunc) contract.Result {
	observe.Event(in.JobID, "worker.started", map[string]any{
		"repository_url": in.RepositoryURL,
		"pinned_sha":     in.PinnedSHA,
	})
	result := BuildResult(in.JobID, start, detect)

	if in.RepositoryURL == "" {
		observe.Event(in.JobID, "worker.completed", map[string]any{"mode": "toolchain_only"})
		return result
	}

	dir, err := os.MkdirTemp("", "patchwrit-repo-*")
	if err != nil {
		return fail(result, "internal", err.Error(), start)
	}
	defer os.RemoveAll(dir)

	observe.Event(in.JobID, "checkout.started", map[string]any{
		"repository_url": in.RepositoryURL,
		"pinned_sha":     in.PinnedSHA,
	})
	cloneStart := time.Now()
	info, err := checkout.Clone(ctx, in.RepositoryURL, in.PinnedSHA, dir)
	result.TimingsMs["clone"] = time.Since(cloneStart).Milliseconds()
	if err != nil {
		category := "clone_failed"
		if ctx.Err() != nil {
			category = "timeout"
		}
		observe.Event(in.JobID, "checkout.failed", map[string]any{
			"category":    category,
			"duration_ms": result.TimingsMs["clone"],
		})
		return fail(result, category, err.Error(), start)
	}
	observe.Event(in.JobID, "checkout.completed", map[string]any{
		"checked_out_sha":         info.CheckedOutSHA,
		"pom_files":               len(info.POMFiles),
		"pom_inventory_truncated": info.POMInventoryTruncated,
		"duration_ms":             result.TimingsMs["clone"],
	})

	result.Repository = info

	if len(info.POMFiles) == 0 {
		return fail(result, "invalid_pom", "no pom.xml found in repository", start)
	}

	observe.Event(in.JobID, "maven.started", map[string]any{"pom_files": len(info.POMFiles)})
	mavenStart := time.Now()
	resolution, err := maven.Resolve(ctx, dir, info.POMFiles, info.POMInventoryTruncated, in.JobID)
	result.TimingsMs["maven"] = time.Since(mavenStart).Milliseconds()
	if err != nil {
		if ctx.Err() != nil {
			return fail(result, "timeout", err.Error(), start)
		}
		category, message := splitCategory(err.Error())
		observe.Event(in.JobID, "maven.failed", map[string]any{
			"category":    category,
			"duration_ms": result.TimingsMs["maven"],
		})
		return fail(result, category, message, start)
	}
	result.Dependencies = resolution.Dependencies
	result.Graph = &resolution.Graph

	result.TimingsMs["execution"] = time.Since(start).Milliseconds()
	observe.Event(in.JobID, "worker.completed", map[string]any{
		"dependencies":   len(result.Dependencies),
		"graph_complete": resolution.Graph.Complete,
		"duration_ms":    result.TimingsMs["execution"],
	})
	return result
}

func splitCategory(message string) (string, string) {
	for _, category := range []string{"invalid_pom", "private_artifact", "jdk_incompatible", "network", "timeout", "internal"} {
		if strings.HasPrefix(message, category+":") {
			return category, strings.TrimSpace(strings.TrimPrefix(message, category+":"))
		}
	}
	return "internal", message
}

func fail(result contract.Result, category, message string, start time.Time) contract.Result {
	result.Status = contract.StatusFailed
	result.Errors = []contract.Error{{Category: category, Message: message}}
	result.TimingsMs["execution"] = time.Since(start).Milliseconds()
	return result
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

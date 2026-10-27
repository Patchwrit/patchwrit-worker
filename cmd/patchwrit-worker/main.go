// Command patchwrit-worker is the ephemeral Patchwrit execution worker.
//
// Wiring only: read job input from the environment, build the result,
// print it to stdout as one JSON object, exit 0. All types live in
// internal/contract, detection in internal/toolchain, assembly in
// internal/worker.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Patchwrit/patchwrit-worker/internal/worker"
)

func main() {
	start := time.Now()

	jobID := os.Getenv("PATCHWRIT_JOB_ID")
	if jobID == "" {
		jobID = "local-smoke"
	}

	timeout := 300 * time.Second
	if raw := os.Getenv("PATCHWRIT_TIMEOUT_SECONDS"); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res := worker.Execute(ctx, worker.Input{
		JobID:         jobID,
		RepositoryURL: os.Getenv("PATCHWRIT_REPOSITORY_URL"),
		PinnedSHA:     os.Getenv("PATCHWRIT_PINNED_SHA"),
	}, start, worker.SystemDetect)

	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		fmt.Fprintf(os.Stderr, "patchwrit-worker: encode result: %v\n", err)
		os.Exit(1)
	}

	// Callback mode (on Fly): deliver the same result to Phoenix.
	// stdout above stays as the local/log copy. A failed delivery exits
	// non-zero so Phoenix treats it as a crash, not a reported result.
	if callbackURL := os.Getenv("PATCHWRIT_CALLBACK_URL"); callbackURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		client := &http.Client{Timeout: 15 * time.Second}
		token := os.Getenv("PATCHWRIT_CALLBACK_TOKEN")

		if err := worker.Deliver(ctx, client, callbackURL, token, res); err != nil {
			fmt.Fprintf(os.Stderr, "patchwrit-worker: deliver result: %v\n", err)
			os.Exit(1)
		}
	}
}

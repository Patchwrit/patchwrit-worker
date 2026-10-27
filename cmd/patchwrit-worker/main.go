// Command patchwrit-worker is the ephemeral Patchwrit execution worker.
//
// Wiring only: read job input from the environment, build the result,
// print it to stdout as one JSON object, exit 0. All types live in
// internal/contract, detection in internal/toolchain, assembly in
// internal/worker.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Patchwrit/patchwrit-worker/internal/worker"
)

func main() {
	start := time.Now()

	jobID := os.Getenv("PATCHWRIT_JOB_ID")
	if jobID == "" {
		jobID = "local-smoke"
	}

	res := worker.BuildResult(jobID, start, worker.SystemDetect)

	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		fmt.Fprintf(os.Stderr, "patchwrit-worker: encode result: %v\n", err)
		os.Exit(1)
	}
}

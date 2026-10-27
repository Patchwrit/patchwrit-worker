// Package contract defines the JSON types shared with the Patchwrit API.
// These mirror docs/ANALYSIS_CONTRACT.md (canonical in patchwrit-api).
package contract

// SchemaVersion is the current worker result schema.
const SchemaVersion = 1

// Worker job outcomes. These are deliberately not the API's analysis
// lifecycle statuses; Phoenix maps between the two.
const (
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Environment reports detected toolchain versions. A nil field means the
// tool is absent; it serializes as JSON null, never an empty string.
type Environment struct {
	GoVersion    string  `json:"go_version"`
	GitVersion   *string `json:"git_version"`
	JavaVersion  *string `json:"java_version"`
	MavenVersion *string `json:"maven_version"`
}

// Error is a categorized worker failure. Category is machine-readable;
// Message is for humans.
type Error struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

// Result is the bounded JSON document the worker emits (stdout in smoke
// mode, the Phoenix callback in later milestones).
type Result struct {
	SchemaVersion int              `json:"schema_version"`
	JobID         string           `json:"job_id"`
	Status        string           `json:"status"`
	WorkerVersion string           `json:"worker_version"`
	Environment   Environment      `json:"environment"`
	TimingsMs     map[string]int64 `json:"timings_ms"`
	Errors        []Error          `json:"errors"`
}

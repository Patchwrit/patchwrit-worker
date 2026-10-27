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

// RepositoryInfo reports the pinned checkout the worker actually made.
// It is evidence, not a claim about vulnerabilities.
type RepositoryInfo struct {
	RepositoryURL string   `json:"repository_url"`
	PinnedSHA     string   `json:"pinned_sha"`
	CheckedOutSHA string   `json:"checked_out_sha"`
	POMFiles      []string `json:"pom_files"`
	Modules       []string `json:"modules"`
}

// Dependency is one node of the effective Maven graph, flattened with
// the introduction path that brought it in.
type Dependency struct {
	GroupID       string   `json:"group_id"`
	ArtifactID    string   `json:"artifact_id"`
	Version       string   `json:"version"`
	Scope         string   `json:"scope,omitempty"`
	Depth         int      `json:"depth"`
	IntroducedVia []string `json:"introduced_via"`
}

func (d Dependency) Coordinate() string {
	return d.GroupID + ":" + d.ArtifactID + ":" + d.Version
}

// Result is the bounded JSON document the worker emits (stdout in smoke
// mode, the Phoenix callback in later milestones).
type Result struct {
	SchemaVersion int              `json:"schema_version"`
	JobID         string           `json:"job_id"`
	Status        string           `json:"status"`
	WorkerVersion string           `json:"worker_version"`
	Environment   Environment      `json:"environment"`
	Repository    *RepositoryInfo  `json:"repository,omitempty"`
	Dependencies  []Dependency     `json:"dependencies,omitempty"`
	TimingsMs     map[string]int64 `json:"timings_ms"`
	Errors        []Error          `json:"errors"`
}

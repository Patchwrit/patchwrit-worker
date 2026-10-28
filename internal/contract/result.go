// Package contract defines the JSON types shared with the Patchwrit API.
// These mirror docs/ANALYSIS_CONTRACT.md (canonical in patchwrit-api).
package contract

// SchemaVersion is the current worker result schema. V2: dependencies
// carry module provenance and all introduction paths, and the result
// carries an explicit graph completeness block.
const SchemaVersion = 2

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
	// POMInventoryTruncated is true when the POM walk hit its cap; the
	// inventory (and therefore module discovery) is then unproven.
	POMInventoryTruncated bool `json:"pom_inventory_truncated"`
}

// Dependency is one node of the effective Maven graph for one module,
// flattened with every introduction path that brought it in.
type Dependency struct {
	GroupID    string `json:"group_id"`
	ArtifactID string `json:"artifact_id"`
	Version    string `json:"version"`
	Scope      string `json:"scope,omitempty"`
	Depth      int    `json:"depth"`
	// Module is the owning reactor module coordinate (group:artifact).
	Module string `json:"module"`
	// POMPath is the module's POM path relative to the repository root.
	POMPath           string     `json:"pom_path"`
	IntroductionPaths [][]string `json:"introduction_paths"`
}

// Graph reports dependency-graph completeness explicitly. Partial
// results are a visible state, never a silent success: Complete is
// false whenever a module failed, the dependency cap was hit, or the
// POM inventory was truncated.
type Graph struct {
	Complete              bool     `json:"complete"`
	ModulesTotal          int      `json:"modules_total"`
	ModulesResolved       int      `json:"modules_resolved"`
	ModulesFailed         []string `json:"modules_failed"`
	DependencyLimitHit    bool     `json:"dependency_limit_hit"`
	POMInventoryTruncated bool     `json:"pom_inventory_truncated"`
	// Instrumentation: how much artifact fetching resolution required,
	// and per-module detail (see ModuleStat).
	DownloadedArtifacts int64        `json:"downloaded_artifacts"`
	DownloadedBytes     int64        `json:"downloaded_bytes"`
	Modules             []ModuleStat `json:"modules"`
}

// ModuleStat is one reactor module's resolution instrumentation.
type ModuleStat struct {
	Module       string `json:"module"`
	POMPath      string `json:"pom_path"`
	Status       string `json:"status"`
	DurationMs   int64  `json:"duration_ms"`
	Dependencies int    `json:"dependencies"`
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
	Graph         *Graph           `json:"graph,omitempty"`
	TimingsMs     map[string]int64 `json:"timings_ms"`
	Errors        []Error          `json:"errors"`
}

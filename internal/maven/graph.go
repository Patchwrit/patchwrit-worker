// Package maven resolves the effective dependency graph of a checked-out
// Maven project. Maven and its plugins are untrusted code: the child
// process runs with a scrubbed environment (no callback or cloud
// secrets) and an isolated HOME/local-repository.
package maven

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Patchwrit/patchwrit-worker/internal/contract"
)

const maxDependencies = 2000

type node struct {
	GroupID    string  `json:"groupId"`
	ArtifactID string  `json:"artifactId"`
	Version    string  `json:"version"`
	Scope      string  `json:"scope"`
	Children   []*node `json:"children"`
}

func (n *node) coordinate() string {
	return n.GroupID + ":" + n.ArtifactID + ":" + n.Version
}

// ParseModuleTree flattens one module's dependency-plugin JSON output.
// Every tree occurrence becomes one Dependency carrying a single
// introduction path; AggregateDependencies merges occurrences of the
// same coordinate into one record with all observed paths. Returns the
// tree's root coordinate (group:artifact) so callers can verify the
// tree belongs to the expected module.
func ParseModuleTree(raw []byte, module, pomPath string) ([]contract.Dependency, string, error) {
	var root node
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, "", fmt.Errorf("parse dependency tree: %w", err)
	}

	deps := []contract.Dependency{}
	var walk func(n *node, depth int, via []string)
	walk = func(n *node, depth int, via []string) {
		for _, child := range n.Children {
			dep := contract.Dependency{
				GroupID:           child.GroupID,
				ArtifactID:        child.ArtifactID,
				Version:           child.Version,
				Scope:             child.Scope,
				Depth:             depth,
				Module:            module,
				POMPath:           pomPath,
				IntroductionPaths: [][]string{append([]string{}, via...)},
			}
			deps = append(deps, dep)
			walk(child, depth+1, append(append([]string{}, via...), child.coordinate()))
		}
	}
	walk(&root, 1, []string{root.coordinate()})
	return deps, root.GroupID + ":" + root.ArtifactID, nil
}

// AggregateDependencies merges occurrences by (module, coordinate,
// scope), unioning introduction paths and keeping the minimum depth.
func AggregateDependencies(occurrences []contract.Dependency) []contract.Dependency {
	index := map[string]int{}
	merged := []contract.Dependency{}

	for _, dep := range occurrences {
		key := dep.Module + "|" + dep.Coordinate() + "|" + dep.Scope
		i, seen := index[key]
		if !seen {
			index[key] = len(merged)
			merged = append(merged, dep)
			continue
		}
		existing := &merged[i]
		if dep.Depth < existing.Depth {
			existing.Depth = dep.Depth
		}
		for _, path := range dep.IntroductionPaths {
			if !containsPath(existing.IntroductionPaths, path) {
				existing.IntroductionPaths = append(existing.IntroductionPaths, path)
			}
		}
	}
	return merged
}

func containsPath(paths [][]string, want []string) bool {
	for _, path := range paths {
		if len(path) != len(want) {
			continue
		}
		equal := true
		for i := range path {
			if path[i] != want[i] {
				equal = false
				break
			}
		}
		if equal {
			return true
		}
	}
	return false
}

// ScrubbedEnv removes anything secret-bearing from a child environment:
// callback credentials, cloud tokens, and GitHub tokens must never reach
// Maven or its plugins (docs/PRE_TRIBUNAL_READINESS.md Gate 1).
func ScrubbedEnv(env []string) []string {
	out := []string{}
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(key)
		if strings.Contains(upper, "TOKEN") || strings.Contains(upper, "SECRET") ||
			strings.Contains(upper, "PASSWORD") || strings.HasPrefix(upper, "PATCHWRIT_CALLBACK") ||
			strings.HasPrefix(upper, "FLY_") {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// Categorize maps Maven failure output to contract error categories.
func Categorize(output string) string {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "non-parseable pom") || strings.Contains(lower, "could not read") && strings.Contains(lower, "pom"):
		return "invalid_pom"
	case strings.Contains(lower, "401") || strings.Contains(lower, "403") ||
		strings.Contains(lower, "could not transfer") || strings.Contains(lower, "unknown host"):
		return "private_artifact"
	default:
		return "internal"
	}
}

// Resolution is the outcome of resolving every reactor module:
// the aggregated dependencies plus explicit completeness. A partial
// Resolution (some modules failed, cap hit) is returned without error;
// error is reserved for total resolution failure.
type Resolution struct {
	Dependencies []contract.Dependency
	Graph        contract.Graph
}

// treeFileName is written per module via a relative -DoutputFile, so
// each reactor project produces its own tree (an absolute path is
// overwritten by every module; see worker issue #12).
const treeFileName = "patchwrit-tree.json"

// Resolve runs the dependency plugin once over the reactor with a
// module-relative output file, then collects and verifies one tree
// per inventoried POM. It never runs the project's own build or
// wrapper script.
func Resolve(ctx context.Context, dir string, pomFiles []string, pomTruncated bool) (*Resolution, error) {
	workHome, err := os.MkdirTemp("", "patchwrit-maven-home-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workHome)

	cmd := exec.CommandContext(ctx, "mvn",
		"-B", "-ntp", "-fae",
		"-Dmaven.repo.local="+filepath.Join(workHome, "m2"),
		// Timestamped log lines let us attribute per-module durations.
		"-Dorg.slf4j.simpleLogger.showDateTime=true",
		"-Dorg.slf4j.simpleLogger.dateTimeFormat=HH:mm:ss,SSS",
		"org.apache.maven.plugins:maven-dependency-plugin:3.8.1:tree",
		"-DoutputType=json",
		"-DoutputFile="+filepath.Join("target", treeFileName),
	)
	cmd.Dir = dir
	env := ScrubbedEnv(os.Environ())
	env = append(env, "HOME="+workHome, "MAVEN_OPTS=-Xmx512m")
	cmd.Env = env

	out, runErr := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("timeout: %w", ctx.Err())
	}

	durations := parseModuleDurations(string(out))
	graph := contract.Graph{
		ModulesTotal:          len(pomFiles),
		ModulesFailed:         []string{},
		Modules:               []contract.ModuleStat{},
		POMInventoryTruncated: pomTruncated,
	}
	occurrences := []contract.Dependency{}
	moduleByPOM := map[string]string{}

	for _, pomPath := range pomFiles {
		stat := contract.ModuleStat{POMPath: pomPath, Status: "failed"}
		moduleDir := filepath.Join(dir, filepath.Dir(pomPath))
		raw, err := os.ReadFile(filepath.Join(moduleDir, "target", treeFileName))
		if err != nil {
			graph.ModulesFailed = append(graph.ModulesFailed, pomPath)
			graph.Modules = append(graph.Modules, stat)
			continue
		}
		deps, root, err := ParseModuleTree(raw, "", pomPath)
		if err != nil {
			graph.ModulesFailed = append(graph.ModulesFailed, pomPath)
			graph.Modules = append(graph.Modules, stat)
			continue
		}
		if expected := pomCoordinate(filepath.Join(dir, pomPath)); expected != "" && expected != root {
			graph.ModulesFailed = append(graph.ModulesFailed, pomPath)
			graph.Modules = append(graph.Modules, stat)
			continue
		}
		for i := range deps {
			deps[i].Module = root
		}
		occurrences = append(occurrences, deps...)
		graph.ModulesResolved++
		stat.Module = root
		stat.Status = "resolved"
		stat.DurationMs = durations[artifactOf(root)]
		moduleByPOM[pomPath] = root
		graph.Modules = append(graph.Modules, stat)
	}

	if graph.ModulesResolved == 0 {
		if runErr != nil {
			return nil, fmt.Errorf("%s: %s", Categorize(string(out)), truncate(string(out), 800))
		}
		return nil, fmt.Errorf("internal: no module dependency trees produced")
	}

	deps := AggregateDependencies(occurrences)
	if len(deps) > maxDependencies {
		graph.DependencyLimitHit = true
		deps = deps[:maxDependencies]
	}

	counts := map[string]int{}
	for _, dep := range deps {
		counts[dep.Module]++
	}
	for i := range graph.Modules {
		graph.Modules[i].Dependencies = counts[graph.Modules[i].Module]
	}
	graph.DownloadedArtifacts, graph.DownloadedBytes = repoStats(filepath.Join(workHome, "m2"))

	graph.Complete = len(graph.ModulesFailed) == 0 && !graph.DependencyLimitHit && !pomTruncated
	return &Resolution{Dependencies: deps, Graph: graph}, nil
}

func artifactOf(coordinate string) string {
	parts := strings.SplitN(coordinate, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return coordinate
}

var (
	treeStartPattern = regexp.MustCompile(`^(\d{2}):(\d{2}):(\d{2}),(\d{3}) \[INFO\] --- dependency:3\.8\.1:tree \(default-cli\) @ (\S+) ---`)
	buildEndPattern  = regexp.MustCompile(`^(\d{2}):(\d{2}):(\d{2}),(\d{3}) \[INFO\] BUILD (SUCCESS|FAILURE)`)
)

// parseModuleDurations attributes wall-clock durations to each
// module's dependency-plugin execution using the timestamped Maven
// log: a module runs from its plugin banner until the next module's
// banner (or the BUILD line, for the last module).
func parseModuleDurations(output string) map[string]int64 {
	type mark struct {
		artifact string
		atMs     int64
	}
	marks := []mark{}
	var buildAt int64 = -1

	for _, line := range strings.Split(output, "\n") {
		if m := treeStartPattern.FindStringSubmatch(line); m != nil {
			marks = append(marks, mark{m[5], timestampMs(m)})
		} else if m := buildEndPattern.FindStringSubmatch(line); m != nil {
			buildAt = timestampMs(m)
		}
	}

	durations := map[string]int64{}
	for i, mark := range marks {
		end := buildAt
		if i+1 < len(marks) {
			end = marks[i+1].atMs
		}
		if end >= mark.atMs {
			durations[mark.artifact] = end - mark.atMs
		}
	}
	return durations
}

func timestampMs(match []string) int64 {
	var h, m, s, ms int64
	fmt.Sscanf(match[1], "%d", &h)
	fmt.Sscanf(match[2], "%d", &m)
	fmt.Sscanf(match[3], "%d", &s)
	fmt.Sscanf(match[4], "%d", &ms)
	return ((h*60+m)*60+s)*1000 + ms
}

// repoStats counts downloaded artifacts (files) and bytes in the
// isolated local repository after resolution.
func repoStats(m2 string) (files int64, bytes int64) {
	_ = filepath.WalkDir(m2, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.IsDir() {
			files++
			if info, err := entry.Info(); err == nil {
				bytes += info.Size()
			}
		}
		return nil
	})
	return files, bytes
}

// pomCoordinate reads a POM's group:artifact (group inherited from the
// parent when absent). Returns "" when unreadable.
func pomCoordinate(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var pom struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Parent     struct {
			GroupID string `xml:"groupId"`
		} `xml:"parent"`
	}
	if err := xml.Unmarshal(raw, &pom); err != nil || pom.ArtifactID == "" {
		return ""
	}
	group := pom.GroupID
	if group == "" {
		group = pom.Parent.GroupID
	}
	if group == "" {
		return ""
	}
	return group + ":" + pom.ArtifactID
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[len(s)-max:]
	}
	return s
}

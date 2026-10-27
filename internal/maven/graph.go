// Package maven resolves the effective dependency graph of a checked-out
// Maven project. Maven and its plugins are untrusted code: the child
// process runs with a scrubbed environment (no callback or cloud
// secrets) and an isolated HOME/local-repository.
package maven

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// ParseTree flattens dependency-plugin JSON output, recording for every
// dependency the chain of coordinates that introduced it.
func ParseTree(raw []byte) ([]contract.Dependency, error) {
	var root node
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse dependency tree: %w", err)
	}

	deps := []contract.Dependency{}
	var walk func(n *node, depth int, via []string)
	walk = func(n *node, depth int, via []string) {
		for _, child := range n.Children {
			if len(deps) >= maxDependencies {
				return
			}
			dep := contract.Dependency{
				GroupID:       child.GroupID,
				ArtifactID:    child.ArtifactID,
				Version:       child.Version,
				Scope:         child.Scope,
				Depth:         depth,
				IntroducedVia: append([]string{}, via...),
			}
			deps = append(deps, dep)
			walk(child, depth+1, append(append([]string{}, via...), child.coordinate()))
		}
	}
	walk(&root, 1, []string{root.coordinate()})
	return deps, nil
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

// Resolve runs the dependency plugin at dir and returns the flattened
// graph. It never runs the project's own build or wrapper script.
func Resolve(ctx context.Context, dir string) ([]contract.Dependency, error) {
	workHome, err := os.MkdirTemp("", "patchwrit-maven-home-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workHome)

	outFile := filepath.Join(workHome, "dependency-tree.json")
	cmd := exec.CommandContext(ctx, "mvn",
		"-B", "-ntp",
		"-Dmaven.repo.local="+filepath.Join(workHome, "m2"),
		"org.apache.maven.plugins:maven-dependency-plugin:3.8.1:tree",
		"-DoutputType=json",
		"-DoutputFile="+outFile,
	)
	cmd.Dir = dir
	env := ScrubbedEnv(os.Environ())
	env = append(env, "HOME="+workHome, "MAVEN_OPTS=-Xmx512m")
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("timeout: %w", ctx.Err())
		}
		return nil, fmt.Errorf("%s: %s", Categorize(string(out)), truncate(string(out), 800))
	}

	raw, err := os.ReadFile(outFile)
	if err != nil {
		return nil, fmt.Errorf("internal: read dependency tree output: %w", err)
	}
	return ParseTree(raw)
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[len(s)-max:]
	}
	return s
}

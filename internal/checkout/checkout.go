// Package checkout clones a repository at an immutable SHA and reports
// the Maven structure it finds. Nothing here executes repository code.
package checkout

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Patchwrit/patchwrit-worker/internal/contract"
)

const maxPOMFiles = 100

// Clone fetches repositoryURL into dir, checks out pinnedSHA, verifies
// HEAD matches it exactly, and inventories POM files and root modules.
func Clone(ctx context.Context, repositoryURL, pinnedSHA, dir string) (*contract.RepositoryInfo, error) {
	if err := git(ctx, "", "clone", "--no-checkout", repositoryURL, dir); err != nil {
		return nil, fmt.Errorf("clone: %w", err)
	}
	if err := git(ctx, dir, "checkout", "--detach", pinnedSHA); err != nil {
		return nil, fmt.Errorf("checkout pinned SHA: %w", err)
	}

	head, err := gitOutput(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("verify HEAD: %w", err)
	}
	if head != pinnedSHA {
		return nil, fmt.Errorf("checked out %s, want pinned %s", head, pinnedSHA)
	}

	poms, truncated, err := findPOMs(dir)
	if err != nil {
		return nil, err
	}

	return &contract.RepositoryInfo{
		RepositoryURL:         repositoryURL,
		PinnedSHA:             pinnedSHA,
		CheckedOutSHA:         head,
		POMFiles:              poms,
		Modules:               rootModules(dir),
		POMInventoryTruncated: truncated,
	}, nil
}

func git(ctx context.Context, dir string, args ...string) error {
	_, err := gitOutput(ctx, dir, args...)
	return err
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// findPOMs inventories pom.xml files, reporting truncated=true when
// the cap stopped the walk early (the inventory is then incomplete
// and must not be presented as exhaustive).
func findPOMs(root string) (poms []string, truncated bool, err error) {
	poms = []string{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if !entry.IsDir() && entry.Name() == "pom.xml" {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			poms = append(poms, filepath.ToSlash(rel))
			if len(poms) >= maxPOMFiles {
				truncated = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("find POM files: %w", err)
	}
	sort.Strings(poms)
	return poms, truncated, nil
}

type pomModules struct {
	Modules []string `xml:"modules>module"`
}

func rootModules(root string) []string {
	raw, err := os.ReadFile(filepath.Join(root, "pom.xml"))
	if err != nil {
		return []string{}
	}
	var pom pomModules
	if err := xml.Unmarshal(raw, &pom); err != nil || pom.Modules == nil {
		return []string{}
	}
	return pom.Modules
}

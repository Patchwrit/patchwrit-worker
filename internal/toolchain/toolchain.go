// Package toolchain detects the versions of tools available to the worker.
package toolchain

import (
	"os/exec"
	"runtime"
	"strings"

	"github.com/Patchwrit/patchwrit-worker/internal/contract"
)

// VersionRunner reports the first line of a tool's version output.
// It returns false when the tool is missing or fails to run.
type VersionRunner func(name string, args ...string) (string, bool)

// ExecRunner is the production VersionRunner backed by os/exec.
func ExecRunner(name string, args ...string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}

	// CombinedOutput: `java -version` writes to stderr, not stdout.
	out, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return "", false
	}

	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	if line == "" {
		return "", false
	}
	return line, true
}

// Detect reports the environment using the given runner.
func Detect(run VersionRunner) contract.Environment {
	return contract.Environment{
		GoVersion:    runtime.Version(),
		GitVersion:   versionPtr(run, "git", "--version"),
		JavaVersion:  versionPtr(run, "java", "-version"),
		MavenVersion: versionPtr(run, "mvn", "-version"),
	}
}

func versionPtr(run VersionRunner, name string, args ...string) *string {
	if version, ok := run(name, args...); ok {
		return &version
	}
	return nil
}

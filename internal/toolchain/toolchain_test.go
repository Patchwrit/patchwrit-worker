package toolchain

import "testing"

func TestDetectDistinguishesPresentAndMissingTools(t *testing.T) {
	runner := func(name string, args ...string) (string, bool) {
		if name == "git" {
			return "git version 2.47.0", true
		}
		return "", false
	}

	env := Detect(runner)

	if env.GoVersion == "" {
		t.Fatal("go_version must always be reported")
	}
	if env.GitVersion == nil || *env.GitVersion != "git version 2.47.0" {
		t.Fatalf("git_version = %v, want detected version", env.GitVersion)
	}
	if env.JavaVersion != nil || env.MavenVersion != nil {
		t.Fatal("missing tools must be nil")
	}
}

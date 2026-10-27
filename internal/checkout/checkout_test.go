package checkout

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestCloneVerifiesPinnedSHAAndFindsPOMs(t *testing.T) {
	src := t.TempDir()
	run(t, src, "init")
	pom := "<project><modules><module>child</module></modules></project>"
	if err := os.WriteFile(filepath.Join(src, "pom.xml"), []byte(pom), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "child", "pom.xml"), []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, src, "add", ".")
	run(t, src, "commit", "-m", "init")
	sha := run(t, src, "rev-parse", "HEAD")
	sha = sha[:len(sha)-1]

	dst := filepath.Join(t.TempDir(), "repo")
	info, err := Clone(context.Background(), src, sha, dst)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if info.CheckedOutSHA != sha || info.PinnedSHA != sha {
		t.Fatalf("sha mismatch: %+v want %s", info, sha)
	}
	if len(info.POMFiles) != 2 || info.POMFiles[0] != "child/pom.xml" || info.POMFiles[1] != "pom.xml" {
		t.Fatalf("pom files = %v", info.POMFiles)
	}
	if len(info.Modules) != 1 || info.Modules[0] != "child" {
		t.Fatalf("modules = %v", info.Modules)
	}
}

func TestCloneRejectsWrongSHA(t *testing.T) {
	src := t.TempDir()
	run(t, src, "init")
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, src, "add", ".")
	run(t, src, "commit", "-m", "init")

	dst := filepath.Join(t.TempDir(), "repo")
	_, err := Clone(context.Background(), src, "0000000000000000000000000000000000000000", dst)
	if err == nil {
		t.Fatal("Clone must fail when the pinned SHA cannot be checked out")
	}
}

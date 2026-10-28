package maven

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Patchwrit/patchwrit-worker/internal/contract"
)

const treeJSON = `{
  "groupId": "com.example", "artifactId": "app", "version": "1.0",
  "children": [
    {"groupId": "org.springframework.boot", "artifactId": "spring-boot-starter-web", "version": "3.3.0", "scope": "compile",
     "children": [
       {"groupId": "org.apache.tomcat.embed", "artifactId": "tomcat-embed-core", "version": "10.1.24", "scope": "compile", "children": []}
     ]},
    {"groupId": "junit", "artifactId": "junit", "version": "4.13.2", "scope": "test", "children": []}
  ]
}`

func TestParseModuleTreeFlattensWithIntroductionPaths(t *testing.T) {
	deps, root, err := ParseModuleTree([]byte(treeJSON), "com.example:app", "pom.xml")
	if err != nil {
		t.Fatalf("ParseModuleTree: %v", err)
	}
	if root != "com.example:app" {
		t.Fatalf("root = %s, want com.example:app", root)
	}
	if len(deps) != 3 {
		t.Fatalf("deps = %d, want 3", len(deps))
	}

	tomcat := deps[1]
	if tomcat.GroupID != "org.apache.tomcat.embed" || tomcat.Version != "10.1.24" {
		t.Fatalf("tomcat dep = %+v", tomcat)
	}
	if tomcat.Depth != 2 {
		t.Fatalf("tomcat depth = %d, want 2", tomcat.Depth)
	}
	want := "com.example:app:1.0 > org.springframework.boot:spring-boot-starter-web:3.3.0"
	if len(tomcat.IntroductionPaths) != 1 || strings.Join(tomcat.IntroductionPaths[0], " > ") != want {
		t.Fatalf("introduction_paths = %v, want [%s]", tomcat.IntroductionPaths, want)
	}
	if deps[0].Depth != 1 || deps[2].Scope != "test" {
		t.Fatalf("deps = %+v", deps)
	}
	if tomcat.Module != "com.example:app" || tomcat.POMPath != "pom.xml" {
		t.Fatalf("provenance = %q %q", tomcat.Module, tomcat.POMPath)
	}
}

func TestAggregateMergesDuplicateCoordinatesWithAllPaths(t *testing.T) {
	const dupTree = `{
	  "groupId": "com.example", "artifactId": "app", "version": "1.0",
	  "children": [
	    {"groupId": "com.example", "artifactId": "lib-a", "version": "1.0", "scope": "compile",
	     "children": [
	       {"groupId": "com.google.code.gson", "artifactId": "gson", "version": "2.8.1", "scope": "compile", "children": []}
	     ]},
	    {"groupId": "com.example", "artifactId": "lib-b", "version": "1.0", "scope": "compile",
	     "children": [
	       {"groupId": "com.google.code.gson", "artifactId": "gson", "version": "2.8.1", "scope": "compile", "children": []}
	     ]}
	  ]
	}`

	occurrences, _, err := ParseModuleTree([]byte(dupTree), "com.example:app", "pom.xml")
	if err != nil {
		t.Fatalf("ParseModuleTree: %v", err)
	}

	deps := AggregateDependencies(occurrences)
	if len(deps) != 3 {
		t.Fatalf("deps = %d, want 3 (lib-a, lib-b, gson)", len(deps))
	}
	var gson *contract.Dependency
	for i := range deps {
		if deps[i].ArtifactID == "gson" {
			gson = &deps[i]
		}
	}
	if len(gson.IntroductionPaths) != 2 {
		t.Fatalf("gson paths = %v, want 2 distinct paths", gson.IntroductionPaths)
	}
	if gson.Depth != 2 {
		t.Fatalf("gson depth = %d, want 2 (minimum)", gson.Depth)
	}
}

// Regression for worker issue #12: a two-module reactor must yield
// both modules' graphs with module provenance. Requires Maven; skipped
// where it is absent (runs in the worker Docker image).
func TestResolveTwoModuleReactor(t *testing.T) {
	if _, err := exec.LookPath("mvn"); err != nil {
		t.Skip("mvn not available")
	}

	dir := t.TempDir()
	write := func(path, content string) {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("pom.xml", `<?xml version="1.0" encoding="UTF-8"?>
<project><modelVersion>4.0.0</modelVersion>
<groupId>com.example.fixture</groupId><artifactId>fixture-parent</artifactId><version>1.0</version>
<packaging>pom</packaging><modules><module>mod-a</module><module>mod-b</module></modules></project>`)
	write("mod-a/pom.xml", `<?xml version="1.0" encoding="UTF-8"?>
<project><modelVersion>4.0.0</modelVersion>
<parent><groupId>com.example.fixture</groupId><artifactId>fixture-parent</artifactId><version>1.0</version></parent>
<artifactId>mod-a</artifactId>
<dependencies><dependency><groupId>com.google.guava</groupId><artifactId>guava</artifactId><version>31.1-jre</version></dependency></dependencies></project>`)
	write("mod-b/pom.xml", `<?xml version="1.0" encoding="UTF-8"?>
<project><modelVersion>4.0.0</modelVersion>
<parent><groupId>com.example.fixture</groupId><artifactId>fixture-parent</artifactId><version>1.0</version></parent>
<artifactId>mod-b</artifactId>
<dependencies><dependency><groupId>commons-beanutils</groupId><artifactId>commons-beanutils</artifactId><version>1.9.4</version></dependency></dependencies></project>`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	res, err := Resolve(ctx, dir, []string{"pom.xml", "mod-a/pom.xml", "mod-b/pom.xml"}, false, "job-test")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !res.Graph.Complete || res.Graph.ModulesTotal != 3 || res.Graph.ModulesResolved != 3 {
		t.Fatalf("graph = %+v, want complete 3/3", res.Graph)
	}

	byModule := map[string]bool{}
	for _, dep := range res.Dependencies {
		byModule[dep.Module] = true
		if dep.ArtifactID == "guava" && dep.POMPath != "mod-a/pom.xml" {
			t.Fatalf("guava provenance = %q, want mod-a/pom.xml", dep.POMPath)
		}
		if dep.ArtifactID == "commons-beanutils" && dep.POMPath != "mod-b/pom.xml" {
			t.Fatalf("beanutils provenance = %q, want mod-b/pom.xml", dep.POMPath)
		}
	}
	if !byModule["com.example.fixture:mod-a"] || !byModule["com.example.fixture:mod-b"] {
		t.Fatalf("modules present = %v, want mod-a and mod-b", byModule)
	}
}

func TestStreamingOutputCapturesAndStreams(t *testing.T) {
	var stream bytes.Buffer
	cmd := exec.Command("sh", "-c", "echo out; echo err >&2")
	captured, err := streamingOutputTo(cmd, &stream)
	if err != nil {
		t.Fatalf("streamingOutputTo: %v", err)
	}
	if !strings.Contains(string(captured), "out") || !strings.Contains(string(captured), "err") {
		t.Fatalf("captured = %q, want both streams", captured)
	}
	if stream.String() != string(captured) {
		t.Fatalf("streamed %q != captured %q", stream.String(), string(captured))
	}
}

func TestParseModuleDurations(t *testing.T) {
	log := `04:00:00,000 [INFO] --- dependency:3.8.1:tree (default-cli) @ fixture-parent ---
04:00:02,500 [INFO] --- dependency:3.8.1:tree (default-cli) @ mod-a ---
04:00:12,500 [INFO] --- dependency:3.8.1:tree (default-cli) @ mod-b ---
04:00:13,000 [INFO] BUILD SUCCESS`

	durations := parseModuleDurations(log)
	if durations["fixture-parent"] != 2500 || durations["mod-a"] != 10000 || durations["mod-b"] != 500 {
		t.Fatalf("durations = %v", durations)
	}
}

func TestScrubbedEnvRemovesSecrets(t *testing.T) {
	env := ScrubbedEnv([]string{
		"PATH=/usr/bin",
		"PATCHWRIT_CALLBACK_TOKEN=secret",
		"PATCHWRIT_CALLBACK_URL=https://example",
		"FLY_API_TOKEN=secret",
		"GITHUB_TOKEN=secret",
		"PATCHWRIT_JOB_ID=job-1",
		"HOME=/root",
	})

	joined := strings.Join(env, "\n")
	for _, leaked := range []string{"CALLBACK_TOKEN=secret", "FLY_API_TOKEN", "GITHUB_TOKEN", "CALLBACK_URL=https"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("secret leaked into Maven env: %s", leaked)
		}
	}
	if !strings.Contains(joined, "PATH=/usr/bin") || !strings.Contains(joined, "PATCHWRIT_JOB_ID=job-1") {
		t.Fatalf("non-secret env must survive: %v", env)
	}
}

func TestCategorize(t *testing.T) {
	if got := Categorize("Non-parseable POM: blah"); got != "invalid_pom" {
		t.Fatalf("got %s", got)
	}
	if got := Categorize("Could not resolve artifact, status: 401"); got != "private_artifact" {
		t.Fatalf("got %s", got)
	}
	if got := Categorize("something exploded"); got != "internal" {
		t.Fatalf("got %s", got)
	}
}

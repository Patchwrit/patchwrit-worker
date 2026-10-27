package maven

import (
	"strings"
	"testing"
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

func TestParseTreeFlattensWithIntroductionPaths(t *testing.T) {
	deps, err := ParseTree([]byte(treeJSON))
	if err != nil {
		t.Fatalf("ParseTree: %v", err)
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
	if strings.Join(tomcat.IntroducedVia, " > ") != want {
		t.Fatalf("introduced_via = %v, want %s", tomcat.IntroducedVia, want)
	}
	if deps[0].Depth != 1 || deps[2].Scope != "test" {
		t.Fatalf("deps = %+v", deps)
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

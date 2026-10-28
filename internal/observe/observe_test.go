package observe

import (
	"bytes"
	"strings"
	"testing"
)

func TestEventFormatIsStableKeyValue(t *testing.T) {
	var buf bytes.Buffer
	previous := Writer
	Writer = &buf
	defer func() { Writer = previous }()

	Event("job-1", "maven.module.completed", map[string]any{
		"module":       "com.example:mod-a",
		"duration_ms":  int64(123),
		"dependencies": 42,
	})

	line := buf.String()
	for _, want := range []string{
		"patchwrit event=maven.module.completed",
		"job_id=job-1",
		"module=com.example:mod-a",
		"duration_ms=123",
		"dependencies=42",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q missing %q", line, want)
		}
	}
	if strings.Count(line, "\n") != 1 {
		t.Fatalf("event must be exactly one line, got %q", line)
	}
}

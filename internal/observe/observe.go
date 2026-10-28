// Package observe emits Patchwrit's shared structured log events.
//
// One line per event, key=value pairs, always to stderr so stdout
// stays reserved for the result JSON:
//
//	patchwrit event=maven.module.completed job_id=<id> module=<g:a> duration_ms=123 dependencies=42
//
// The Phoenix side emits the same event vocabulary with the same
// field names (docs/LOGGING.md in patchwrit-api). Human/toolchain
// output (Maven, Git) also streams to stderr unchanged; these lines
// are the greppable spine inside it. Never log tokens or secrets.
package observe

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
)

// Writer is the event sink. Tests replace it; production is stderr.
var Writer io.Writer = os.Stderr

var mu sync.Mutex

// Event writes one structured event line. Fields are sorted by key
// for stable, greppable output.
func Event(jobID, event string, fields map[string]any) {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("patchwrit event=")
	b.WriteString(event)
	if jobID != "" {
		b.WriteString(" job_id=")
		b.WriteString(jobID)
	}
	for _, key := range keys {
		fmt.Fprintf(&b, " %s=%v", key, fields[key])
	}
	b.WriteString("\n")

	mu.Lock()
	defer mu.Unlock()
	_, _ = io.WriteString(Writer, b.String())
}

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// The transcript recorder writes every request this suite sends and every
// response it gets back to a single file, so the wire traffic can be reviewed
// against BCA's field tables without re-reading the tests or re-running them.
//
// It is off unless E2E_TRANSCRIPT names an output path:
//
//	E2E_TRANSCRIPT=docs/e2e/e2e-transcript.md go test ./test/e2e/...
//
// A relative path is resolved against the repository root, not the package
// directory.
//
// Off by default on purpose. Every run stamps fresh timestamps, X-EXTERNAL-IDs
// and signatures into the file, so writing it unconditionally would make an
// ordinary `go test ./...` dirty the working tree.

type transcriptEntry struct {
	test     string
	source   string
	method   string
	path     string
	reqHead  http.Header
	reqBody  string
	status   int
	respHead http.Header
	respBody string
}

type transcriptRecorder struct {
	mu      sync.Mutex
	path    string
	entries []transcriptEntry
}

var transcript = &transcriptRecorder{path: transcriptPath(os.Getenv("E2E_TRANSCRIPT"))}

// transcriptPath resolves a relative E2E_TRANSCRIPT against the repository
// root rather than the package directory. `go test` runs each package in its
// own directory, so an unresolved relative path would drop the transcript in
// test/e2e/ instead of where the caller meant.
func transcriptPath(configured string) string {
	if configured == "" || filepath.IsAbs(configured) {
		return configured
	}
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return configured
	}
	return filepath.Join(strings.TrimSpace(string(root)), configured)
}

func (r *transcriptRecorder) enabled() bool { return r != nil && r.path != "" }

// record is called from server.call with the request as it went on the wire and
// the recorder it was answered into.
func (r *transcriptRecorder) record(t *testing.T, req *http.Request, body string, rec *httptest.ResponseRecorder) {
	if !r.enabled() {
		return
	}

	// The test file the call came from, which is how the transcript is
	// sectioned: flows, negative cases, multi-tenant, conformance.
	source := "unknown"
	for depth := 2; depth < 8; depth++ {
		_, file, _, ok := runtime.Caller(depth)
		if !ok {
			break
		}
		if base := filepath.Base(file); strings.HasSuffix(base, "_test.go") && base != "harness_test.go" && base != "transcript_test.go" {
			source = base
			break
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, transcriptEntry{
		test:     t.Name(),
		source:   source,
		method:   req.Method,
		path:     req.URL.Path,
		reqHead:  req.Header.Clone(),
		reqBody:  body,
		status:   rec.Code,
		respHead: rec.Header().Clone(),
		respBody: rec.Body.String(),
	})
}

// sourceTitles gives each test file a heading; anything unlisted keeps its
// filename.
var sourceTitles = map[string]string{
	"transaction_flows_test.go":      "Transaction flows — fixed bill, variable bill, no bill",
	"negative_cases_test.go":         "Negative cases — auth, headers, payload, business rejections",
	"multi_tenant_test.go":           "Multi-vendor / multi-merchant isolation",
	"va_bca_conformance_e2e_test.go": "BCA conformance regressions",
}

var sourceOrder = []string{
	"transaction_flows_test.go",
	"negative_cases_test.go",
	"multi_tenant_test.go",
	"va_bca_conformance_e2e_test.go",
}

func (r *transcriptRecorder) flush() error {
	if !r.enabled() {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if dir := filepath.Dir(r.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	var b strings.Builder
	b.WriteString("# SNAP Virtual Account — end-to-end request/response transcript\n\n")
	b.WriteString("Every request this suite puts on the wire and every response it got back,\n")
	b.WriteString("captured from an actual run of `test/e2e`. The suite drives the production\n")
	b.WriteString("router, idempotency middleware, SNAP auth middleware, handler and usecase\n")
	b.WriteString("against an in-memory repository, so the headers, `stringToSign` inputs,\n")
	b.WriteString("service codes and JSON envelopes below are the real ones.\n\n")
	b.WriteString("Regenerate with:\n\n")
	b.WriteString("```sh\nE2E_TRANSCRIPT=docs/e2e/e2e-transcript.md go test ./test/e2e/...\n```\n\n")
	fmt.Fprintf(&b, "- Generated: `%s`\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "- Commit: `%s`\n", gitDescribe())
	fmt.Fprintf(&b, "- Exchanges: %d across %d scenarios\n\n", len(r.entries), countTests(r.entries))
	b.WriteString("Signatures and timestamps are genuine but computed over the suite's own\n")
	b.WriteString("throwaway vendor secret, so they change on every run.\n\n")

	b.WriteString("## Contents\n\n")
	for _, source := range r.orderedSources() {
		fmt.Fprintf(&b, "- [%s](#%s)\n", titleFor(source), anchor(titleFor(source)))
	}
	b.WriteString("\n")

	for _, source := range r.orderedSources() {
		fmt.Fprintf(&b, "---\n\n## %s\n\n", titleFor(source))

		lastTest := ""
		seq := 0
		for _, e := range r.entries {
			if e.source != source {
				continue
			}
			if e.test != lastTest {
				fmt.Fprintf(&b, "### %s\n\n", e.test)
				lastTest = e.test
				seq = 0
			}
			seq++
			fmt.Fprintf(&b, "#### %d. `%s %s` → %d\n\n", seq, e.method, e.path, e.status)

			b.WriteString("**Request**\n\n```http\n")
			fmt.Fprintf(&b, "%s %s HTTP/1.1\n", e.method, e.path)
			writeHeaders(&b, e.reqHead)
			b.WriteString("\n")
			b.WriteString(prettyBody(e.reqBody))
			b.WriteString("\n```\n\n")

			b.WriteString("**Response**\n\n```http\n")
			fmt.Fprintf(&b, "HTTP/1.1 %d %s\n", e.status, http.StatusText(e.status))
			writeHeaders(&b, e.respHead)
			b.WriteString("\n")
			b.WriteString(prettyBody(e.respBody))
			b.WriteString("\n```\n\n")
		}
	}

	return os.WriteFile(r.path, []byte(b.String()), 0o644)
}

// orderedSources lists the files that actually produced traffic, known ones
// first in reading order.
func (r *transcriptRecorder) orderedSources() []string {
	present := map[string]bool{}
	for _, e := range r.entries {
		present[e.source] = true
	}
	var out []string
	for _, source := range sourceOrder {
		if present[source] {
			out = append(out, source)
			delete(present, source)
		}
	}
	rest := make([]string, 0, len(present))
	for source := range present {
		rest = append(rest, source)
	}
	sort.Strings(rest)
	return append(out, rest...)
}

func titleFor(source string) string {
	if title, ok := sourceTitles[source]; ok {
		return title
	}
	return source
}

func anchor(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-':
			b.WriteRune('-')
		}
	}
	return b.String()
}

func countTests(entries []transcriptEntry) int {
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.test] = true
	}
	return len(seen)
}

// wireCasing restores the spelling BCA publishes for the SNAP headers. Go
// canonicalises header names on the way in ("X-EXTERNAL-ID" becomes
// "X-External-Id"), which is correct over the wire — header names are
// case-insensitive — but in a conformance transcript it reads as though this
// service sends something other than what the documentation specifies.
var wireCasing = map[string]string{
	"X-Timestamp":     "X-TIMESTAMP",
	"X-Signature":     "X-SIGNATURE",
	"X-Partner-Id":    "X-PARTNER-ID",
	"X-External-Id":   "X-EXTERNAL-ID",
	"Channel-Id":      "CHANNEL-ID",
	"Origin":          "ORIGIN",
	"X-Client-Key":    "X-CLIENT-KEY",
	"Idempotency-Key": "Idempotency-Key",
}

func writeHeaders(b *strings.Builder, h http.Header) {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		wire := name
		if canonical, ok := wireCasing[name]; ok {
			wire = canonical
		}
		for _, value := range h[name] {
			fmt.Fprintf(b, "%s: %s\n", wire, value)
		}
	}
}

// prettyBody indents JSON so the field set is readable, and leaves anything
// that is not JSON — the malformed-body cases — exactly as it went on the wire.
func prettyBody(body string) string {
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return "(empty)"
	}
	var indented strings.Builder
	if err := indentJSON(&indented, body); err != nil {
		return body
	}
	return indented.String()
}

func indentJSON(dst *strings.Builder, body string) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(body), "", "  "); err != nil {
		return err
	}
	dst.Write(buf.Bytes())
	return nil
}

func gitDescribe() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func TestMain(m *testing.M) {
	code := m.Run()
	if err := transcript.flush(); err != nil {
		fmt.Fprintf(os.Stderr, "transcript: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

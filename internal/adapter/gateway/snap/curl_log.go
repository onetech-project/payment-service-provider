package snap

import (
	"net/http"
	"sort"
	"strings"
)

// Header names whose values are credentials rather than request facts.
//
// Authorization carries the bearer token: one value, valid ~900s, accepted on
// every service this vendor exposes. Logging it would put a working
// credential into Loki, where it is retained, queryable, and readable by
// anyone with Grafana access — so it is replaced everywhere, including in the
// reproduction command below.
//
// X-SIGNATURE is deliberately NOT in this set. It is a MAC over this one
// request's stringToSign: it does not reveal the shared secret, it is useless
// without the bearer token that is redacted here, and being able to compare it
// against an independently computed signature is most of the value of logging
// the request at all.
var redactedHeaders = map[string]bool{
	"Authorization": true,
}

// headerPlaceholders gives each redacted header a shell variable to stand in
// for it, so the logged curl is complete apart from the secret and can be run
// by exporting that one value.
var headerPlaceholders = map[string]string{
	"Authorization": "Bearer $BCA_ACCESS_TOKEN",
}

// loggableHeaders renders the request's headers as "Name: value" lines, sorted
// by name so two log entries can be diffed against each other. Credential
// values are replaced, never truncated — a prefix of a token is still a leak
// of part of it, and a length is enough to confirm presence.
func loggableHeaders(h http.Header) []string {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]string, 0, len(names))
	for _, name := range names {
		value := h.Get(name)
		if redactedHeaders[http.CanonicalHeaderKey(name)] {
			value = "<redacted>"
		}
		out = append(out, name+": "+value)
	}
	return out
}

// curlCommand renders the request as a runnable curl invocation.
//
// It exists because reproducing a SNAP call by hand is error-prone in exactly
// the ways that break it: the signature is computed over the body, so
// re-typing the JSON changes the hash, and a header dropped by accident
// (X-EXTERNAL-ID, say) fails for a reason the vendor will not name. Copying
// the command out of the log removes that whole class of mistake when
// comparing a failing service call against a working manual one.
//
// The bearer token is a placeholder, so the command is complete except for the
// one value that must not be written down:
//
//	export BCA_ACCESS_TOKEN=...   # from scripts/curl-b2b-token.sh
//
// Single quotes are the quoting form throughout, with any embedded quote
// closed and reopened ('"'"'), which is safe for arbitrary header and body
// content.
func curlCommand(method, url string, h http.Header, body []byte) string {
	var b strings.Builder
	b.WriteString("curl -sS -X ")
	b.WriteString(method)
	b.WriteString(" ")
	b.WriteString(shellQuote(url))

	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		value := h.Get(name)
		if placeholder, ok := headerPlaceholders[http.CanonicalHeaderKey(name)]; ok {
			value = placeholder
		} else if redactedHeaders[http.CanonicalHeaderKey(name)] {
			value = "<redacted>"
		}
		b.WriteString(" -H ")
		b.WriteString(shellQuote(name + ": " + value))
	}

	if len(body) > 0 {
		b.WriteString(" -d ")
		b.WriteString(shellQuote(string(body)))
	}
	return b.String()
}

// shellQuote wraps s in single quotes for /bin/sh, escaping any single quote
// it contains. Used rather than %q: Go's quoting is not shell quoting, and a
// body containing a backslash would come out of %q altered.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

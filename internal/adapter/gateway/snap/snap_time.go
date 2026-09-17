package snap

import "time"

// snapTimestampLayout is RFC3339 with the offset always written out, never
// collapsed to "Z".
//
// Go's time.RFC3339 emits "Z" for UTC, which is valid RFC3339 but not what BCA
// accepts: the timestamp is a component of stringToSign, so BCA rebuilds the
// string from the literal header value and computes a different signature than
// we did. Its gateway answers that mismatch with a 504 Timeout rather than a
// signature error — the same misleading response every other malformed-request
// cause produces here.
const snapTimestampLayout = "2006-01-02T15:04:05-07:00"

// wib is Western Indonesian Time, the zone BCA's API operates in.
//
// A fixed zone rather than time.LoadLocation("Asia/Jakarta"): Indonesia has
// observed no DST since 1964, so the offset is a constant, and a fixed zone
// cannot fail at runtime the way a tzdata lookup can in a container whose
// zoneinfo is absent or trimmed. LoadLocation would return an error there, and
// the fallback would be exactly the UTC rendering this exists to avoid.
var wib = time.FixedZone("WIB", 7*60*60)

// snapTimestamp formats t for X-TIMESTAMP as BCA expects it: RFC3339 in
// +07:00, whatever zone the host clock is set to.
//
// Converting rather than just formatting is the point. The instant is the same
// either way and "+00:00" would be arithmetically correct, but every other
// party reading this value — BCA's own logs, their support staff, the samples
// in their documentation — works in WIB, so a UTC rendering makes the service's
// requests describe a different wall-clock time than everything they are
// compared against.
func snapTimestamp(t time.Time) string {
	return t.In(wib).Format(snapTimestampLayout)
}

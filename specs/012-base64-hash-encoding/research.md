# Research: Base64 Hash/Signature Encoding Standardization

## Decision 1: Rename `HashSHA256Hex` vs. add a parallel `HashSHA256Base64` and keep both

**Decision**: Rename/re-implement `HashSHA256Hex` → `HashSHA256Base64` (and `HashSHA256Reader` → `HashSHA256ReaderBase64`), updating all 4 call sites in the same change, rather than adding a new function alongside the old one.

**Rationale**: Spec FR-003/Assumptions state this is an intentional breaking change with no dual-encoding compatibility mode. Keeping `HashSHA256Hex` around (even unused) would invite accidental reintroduction of hex encoding at a new call site later, and violates YAGNI — there's no scenario in this codebase where both encodings need to coexist post-migration. A clean rename also makes `go build` fail at every stale call site until each is updated, which is a useful forcing function for completeness (all 4 callers must be touched, not just the "main" ones).

**Alternatives considered**:
- *Add `HashSHA256Base64` and leave `HashSHA256Hex` in place, migrate callers gradually*: rejected — leaves dead/misleading code and reintroduces exactly the kind of "two conventions in one codebase" problem this feature exists to eliminate (echoing the concern already raised in feature 011's research about symmetric vs. asymmetric secrets).

## Decision 2: `HMACSigner.Sign`/`Verify` — reuse the existing `SignBase64` or make base64 the default

**Decision**: Make base64 the default behavior of `Sign`/`Verify` (i.e., `Sign` gets `SignBase64`'s current implementation; the old hex-producing body is deleted, not kept as a second method), rather than switching every caller from `Sign` to the already-existing `SignBase64`.

**Rationale**: `SignBase64` already exists in `hmac.go` today but is unused anywhere in the codebase (confirmed via grep) — it's dead code, likely added in anticipation of exactly this kind of change. Renaming/repurposing `Sign` itself (rather than migrating callers to call `SignBase64` instead) means `Verify` — which internally calls `s.Sign`, per `hmac.go:55` — automatically verifies against base64 without a separate edit, and no caller-side code needs to change its method name, only its expectations of what value is returned by an existing call it already makes. This is fewer edits and less risk of missing a call site than migrating every caller from `.Sign(...)` to `.SignBase64(...)`.

**Alternatives considered**:
- *Keep `Sign` hex, migrate all callers to call the existing `SignBase64` instead*: rejected — more call sites to touch (`signature_usecase.go`, `snap/client.go`, both middleware files' `Verify` calls indirectly), and leaves a confusingly-named `Sign` (hex) vs `SignBase64` (base64) pair when only one encoding is ever used going forward.

## Decision 3: Idempotency payload hash and webhook signature — migrate in the same feature or split out

**Decision**: Migrate both in this same feature (per the user's explicit scope choice — "semua di atas"), each as its own independently-testable user story (P3 and P2 respectively), rather than a separate feature/PR.

**Rationale**: Both are simple, self-contained `hex.EncodeToString` → `base64.StdEncoding.EncodeToString` swaps in `idempotency.go:76` and `payment_notification_worker.go:102`, with no shared code path with the `crypto` package changes (they hand-roll their own hashing rather than calling into `crypto.HashSHA256Hex`). Splitting them into separate features would add process overhead (separate spec/plan/tasks cycles) for two one-line changes that carry no additional design complexity beyond "same swap, different file."

**Alternatives considered**:
- *Defer idempotency/webhook changes to a follow-up feature*: rejected per explicit user scope decision in the spec's Clarifications section; also rejected on its own merits since these are trivial, low-risk, single-line changes that don't need separate planning overhead.

## Decision 4: Shell script signature computation — `openssl dgst -hex` replacement

**Decision**: Replace `openssl dgst -sha256 -hex | awk '{print $NF}'` with `openssl dgst -sha256 -binary | openssl base64 -A` (or equivalent single-pipeline base64 encode), and similarly for the `-sha512 -hmac` signature computation, across all 5 affected scripts.

**Rationale**: `openssl dgst` supports `-binary` output which can be piped directly into `openssl base64 -A` (no line wrapping) to get a single-line base64 string — the same shape as the current `awk '{print $NF}'` hex extraction, so the surrounding script logic (variable assignment, header construction) needs no other changes. This mirrors the exact tool (`openssl`) already used, avoiding a new dependency (e.g. `base64` coreutils vs. `openssl base64` — sticking with `openssl` for consistency since it's already required by every affected script).

**Alternatives considered**:
- *Use the `base64` coreutil instead of `openssl base64`*: rejected — no functional difference, but `openssl` is already the documented required tool for every affected script (per each script's header comment), so introducing a second tool for the same job adds an unnecessary dependency line to update.

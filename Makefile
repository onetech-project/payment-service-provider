.PHONY: swagger test test-cover coverage-html coverage-inscope

# Packages deliberately excluded from the "in-scope" coverage figure: thin
# infra wrappers (DB/Redis/queue/telemetry) that only call SDKs directly and
# would need testcontainers to test meaningfully, plus generated docs and
# main/wiring code. Kept in sync with sonar.coverage.exclusions in
# sonar-project.properties so `make coverage-inscope` and the SonarQube
# coverage % match.
COVERAGE_EXCLUDE := internal/infrastructure/database/|internal/infrastructure/redis/|internal/infrastructure/queue/|internal/infrastructure/telemetry/|/docs/|cmd/api/

# Regenerates docs/ (docs.go, swagger.json, swagger.yaml) from the
# swaggo annotations above cmd/api/main.go and the handler functions in
# internal/adapter/delivery/http/handler/*.go. Run this after editing
# any @Summary/@Param/@Success/@Failure/@Router annotation.
swagger:
	swag init -g cmd/api/main.go --output docs

# Runs the unit test suite.
test:
	go test ./... -v

# Runs the unit test suite with coverage instrumentation and prints the
# per-function + total coverage summary. coverage.out is consumed by
# sonar-project.properties (sonar.go.coverage.reportPaths) for SonarQube.
test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

# Renders coverage.out as an HTML report for local inspection.
coverage-html: test-cover
	go tool cover -html=coverage.out -o coverage.html

# Prints the total coverage % for business logic only, excluding the raw
# infra wrappers/docs/wiring listed in COVERAGE_EXCLUDE (same scope Sonar is
# configured to measure). Use this number, not the raw `test-cover` total,
# to judge whether the code that's actually unit-testable is well covered.
coverage-inscope: test-cover
	@{ head -1 coverage.out; grep -Ev '$(COVERAGE_EXCLUDE)' coverage.out | tail -n +2; } > coverage.inscope.out
	@go tool cover -func=coverage.inscope.out | tail -1

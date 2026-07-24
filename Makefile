.PHONY: swagger

# Regenerates docs/ (docs.go, swagger.json, swagger.yaml) from the
# swaggo annotations above cmd/api/main.go and the handler functions in
# internal/adapter/delivery/http/handler/*.go. Run this after editing
# any @Summary/@Param/@Success/@Failure/@Router annotation.
swagger:
	swag init -g cmd/api/main.go --output docs

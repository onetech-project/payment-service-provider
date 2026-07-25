package domain_test

import (
	"errors"
	"testing"

	"backbone-new/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestNewDomainError(t *testing.T) {
	wrapped := errors.New("underlying cause")
	err := domain.NewDomainError("4000000", "Bad Request", wrapped)

	assert.Equal(t, "4000000", err.SNAPCode)
	assert.Equal(t, "Bad Request", err.Message)
	assert.Equal(t, wrapped, err.Err)
}

func TestDomainError_Error(t *testing.T) {
	t.Run("with wrapped error", func(t *testing.T) {
		wrapped := errors.New("underlying cause")
		err := domain.NewDomainError("4000000", "Bad Request", wrapped)
		assert.Equal(t, "[4000000] Bad Request: underlying cause", err.Error())
	})

	t.Run("without wrapped error", func(t *testing.T) {
		err := domain.NewDomainError("2000000", "Successful", nil)
		assert.Equal(t, "[2000000] Successful", err.Error())
	})
}

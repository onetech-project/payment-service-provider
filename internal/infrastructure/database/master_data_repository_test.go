package database

import (
	"testing"

	"backbone-new/internal/domain"

	"github.com/stretchr/testify/assert"
)

// TestNewMasterVADataRepository_ImplementsInterface is a compile-time-backed
// smoke test: the SQL-executing methods themselves (ListVATypes,
// CreateVAType, etc.) require a live PostgreSQL connection to exercise
// meaningfully, same limitation already accepted for VARepository in
// va_repository_test.go — those are covered by quickstart.md's live
// scenarios (8/9) instead of a unit test here.
func TestNewMasterVADataRepository_ImplementsInterface(t *testing.T) {
	repo := NewMasterVADataRepository(nil)
	assert.NotNil(t, repo)

	var _ domain.MasterDataRepository = repo
}

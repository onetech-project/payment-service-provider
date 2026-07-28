package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"backbone-new/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) ListVATypes(ctx context.Context) ([]domain.VATypeRule, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.VATypeRule), args.Error(1)
}

func (m *mockRepo) ListPartnerServiceIDs(ctx context.Context) ([]domain.PartnerServiceIDRecord, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.PartnerServiceIDRecord), args.Error(1)
}

func (m *mockRepo) CreateVAType(ctx context.Context, rule domain.VATypeRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *mockRepo) UpdateVAType(ctx context.Context, rule domain.VATypeRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *mockRepo) DeleteVAType(ctx context.Context, vaType string) error {
	args := m.Called(ctx, vaType)
	return args.Error(0)
}

func (m *mockRepo) CreatePartnerServiceID(ctx context.Context, record domain.PartnerServiceIDRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *mockRepo) UpdatePartnerServiceID(ctx context.Context, record domain.PartnerServiceIDRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *mockRepo) DeletePartnerServiceID(ctx context.Context, partnerServiceID string) error {
	args := m.Called(ctx, partnerServiceID)
	return args.Error(0)
}

type mockCache struct {
	mock.Mock
}

func (m *mockCache) GetVATypes(ctx context.Context) ([]domain.VATypeRule, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.VATypeRule), args.Error(1)
}

func (m *mockCache) SetVATypes(ctx context.Context, rules []domain.VATypeRule) error {
	args := m.Called(ctx, rules)
	return args.Error(0)
}

func (m *mockCache) GetPartnerServiceIDs(ctx context.Context) ([]domain.PartnerServiceIDRecord, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.PartnerServiceIDRecord), args.Error(1)
}

func (m *mockCache) SetPartnerServiceIDs(ctx context.Context, records []domain.PartnerServiceIDRecord) error {
	args := m.Called(ctx, records)
	return args.Error(0)
}

var sampleRules = []domain.VATypeRule{
	{VAType: "04", PartnerServiceID: "15973", Dynamic: true, Billing: domain.VATypeBillingNone},
}

var samplePartnerIDs = []domain.PartnerServiceIDRecord{
	{PartnerServiceID: "15973", BankCode: "BANK-15973"},
}

func TestCachedVATypeRuleProvider_LookupVATypeRule_CacheHit(t *testing.T) {
	repo := new(mockRepo)
	c := new(mockCache)
	c.On("GetVATypes", mock.Anything).Return(sampleRules, nil)

	p := NewCachedVATypeRuleProvider(repo, c)
	rule, ok, err := p.LookupVATypeRule(context.Background(), "15973", "04")

	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "04", rule.VAType)
	repo.AssertNotCalled(t, "ListVATypes", mock.Anything)
}

func TestCachedVATypeRuleProvider_LookupVATypeRule_CacheMiss_RefillsFromRepo(t *testing.T) {
	repo := new(mockRepo)
	c := new(mockCache)
	c.On("GetVATypes", mock.Anything).Return(nil, nil)
	repo.On("ListVATypes", mock.Anything).Return(sampleRules, nil)
	c.On("SetVATypes", mock.Anything, sampleRules).Return(nil)

	p := NewCachedVATypeRuleProvider(repo, c)
	rule, ok, err := p.LookupVATypeRule(context.Background(), "15973", "04")

	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "04", rule.VAType)
	c.AssertExpectations(t)
}

func TestCachedVATypeRuleProvider_LookupVATypeRule_NotFound(t *testing.T) {
	repo := new(mockRepo)
	c := new(mockCache)
	c.On("GetVATypes", mock.Anything).Return(sampleRules, nil)

	p := NewCachedVATypeRuleProvider(repo, c)
	_, ok, err := p.LookupVATypeRule(context.Background(), "99999", "99")

	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestCachedVATypeRuleProvider_RefreshNow_PopulatesCacheAndSnapshot(t *testing.T) {
	repo := new(mockRepo)
	c := new(mockCache)
	repo.On("ListVATypes", mock.Anything).Return(sampleRules, nil)
	repo.On("ListPartnerServiceIDs", mock.Anything).Return(samplePartnerIDs, nil)
	c.On("SetVATypes", mock.Anything, sampleRules).Return(nil)
	c.On("SetPartnerServiceIDs", mock.Anything, samplePartnerIDs).Return(nil)

	p := NewCachedVATypeRuleProvider(repo, c)
	err := p.RefreshNow(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, sampleRules, p.snapshotVATypes())
	assert.Equal(t, samplePartnerIDs, p.snapshotPartnerIDs())
}

func TestCachedVATypeRuleProvider_RedisUnavailable_FallsBackToSnapshot(t *testing.T) {
	repo := new(mockRepo)
	c := new(mockCache)
	repo.On("ListVATypes", mock.Anything).Return(sampleRules, nil)
	repo.On("ListPartnerServiceIDs", mock.Anything).Return(samplePartnerIDs, nil)
	c.On("SetVATypes", mock.Anything, sampleRules).Return(nil)
	c.On("SetPartnerServiceIDs", mock.Anything, samplePartnerIDs).Return(nil)

	p := NewCachedVATypeRuleProvider(repo, c)
	assert.NoError(t, p.RefreshNow(context.Background())) // warm the snapshot

	// Now simulate Redis being unreachable for subsequent reads.
	c2 := new(mockCache)
	c2.On("GetVATypes", mock.Anything).Return(nil, errors.New("redis unreachable"))
	p.cache = c2

	rule, ok, err := p.LookupVATypeRule(context.Background(), "15973", "04")

	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "04", rule.VAType)
}

func TestCachedVATypeRuleProvider_IsReservedPartnerServiceID(t *testing.T) {
	repo := new(mockRepo)
	c := new(mockCache)
	c.On("GetPartnerServiceIDs", mock.Anything).Return(samplePartnerIDs, nil)

	p := NewCachedVATypeRuleProvider(repo, c)
	reserved, err := p.IsReservedPartnerServiceID(context.Background(), "15973")
	assert.NoError(t, err)
	assert.True(t, reserved)

	notReserved, err := p.IsReservedPartnerServiceID(context.Background(), "099999")
	assert.NoError(t, err)
	assert.False(t, notReserved)
}

func TestCachedVATypeRuleProvider_StartStop_NoPanicAndTicks(t *testing.T) {
	repo := new(mockRepo)
	c := new(mockCache)
	repo.On("ListVATypes", mock.Anything).Return(sampleRules, nil)
	repo.On("ListPartnerServiceIDs", mock.Anything).Return(samplePartnerIDs, nil)
	c.On("SetVATypes", mock.Anything, mock.Anything).Return(nil)
	c.On("SetPartnerServiceIDs", mock.Anything, mock.Anything).Return(nil)

	p := NewCachedVATypeRuleProvider(repo, c)
	p.Start(context.Background(), time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	p.Stop()
	p.Stop() // idempotent
}

func TestCachedVATypeRuleProvider_MutationPassthroughs_RefreshAfterWrite(t *testing.T) {
	repo := new(mockRepo)
	c := new(mockCache)
	rule := domain.VATypeRule{VAType: "07", PartnerServiceID: "15976", Dynamic: false, Billing: domain.VATypeBillingNone}
	record := domain.PartnerServiceIDRecord{PartnerServiceID: "15976", BankCode: "BANK-15976"}

	repo.On("UpdateVAType", mock.Anything, rule).Return(nil)
	repo.On("DeleteVAType", mock.Anything, "07").Return(nil)
	repo.On("CreatePartnerServiceID", mock.Anything, record).Return(nil)
	repo.On("UpdatePartnerServiceID", mock.Anything, record).Return(nil)
	repo.On("DeletePartnerServiceID", mock.Anything, "15976").Return(nil)
	repo.On("ListVATypes", mock.Anything).Return(sampleRules, nil)
	repo.On("ListPartnerServiceIDs", mock.Anything).Return(samplePartnerIDs, nil)
	c.On("SetVATypes", mock.Anything, mock.Anything).Return(nil)
	c.On("SetPartnerServiceIDs", mock.Anything, mock.Anything).Return(nil)

	p := NewCachedVATypeRuleProvider(repo, c)

	assert.NoError(t, p.UpdateVAType(context.Background(), rule))
	assert.NoError(t, p.DeleteVAType(context.Background(), "07"))
	assert.NoError(t, p.CreatePartnerServiceID(context.Background(), record))
	assert.NoError(t, p.UpdatePartnerServiceID(context.Background(), record))
	assert.NoError(t, p.DeletePartnerServiceID(context.Background(), "15976"))
	repo.AssertExpectations(t)
}

func TestCachedVATypeRuleProvider_MutationPassthrough_RepoErrorSkipsRefresh(t *testing.T) {
	repo := new(mockRepo)
	c := new(mockCache)
	rule := domain.VATypeRule{VAType: "08", PartnerServiceID: "15977"}
	repo.On("CreateVAType", mock.Anything, rule).Return(errors.New("insert failed"))

	p := NewCachedVATypeRuleProvider(repo, c)
	err := p.CreateVAType(context.Background(), rule)

	assert.Error(t, err)
	repo.AssertNotCalled(t, "ListVATypes", mock.Anything)
}

func TestCachedVATypeRuleProvider_CreateVAType_RefreshesAfterWrite(t *testing.T) {
	repo := new(mockRepo)
	c := new(mockCache)
	newRule := domain.VATypeRule{VAType: "07", PartnerServiceID: "15976", Dynamic: false, Billing: domain.VATypeBillingNone}
	repo.On("CreateVAType", mock.Anything, newRule).Return(nil)
	repo.On("ListVATypes", mock.Anything).Return([]domain.VATypeRule{newRule}, nil)
	repo.On("ListPartnerServiceIDs", mock.Anything).Return(samplePartnerIDs, nil)
	c.On("SetVATypes", mock.Anything, mock.Anything).Return(nil)
	c.On("SetPartnerServiceIDs", mock.Anything, mock.Anything).Return(nil)

	p := NewCachedVATypeRuleProvider(repo, c)
	err := p.CreateVAType(context.Background(), newRule)

	assert.NoError(t, err)
	assert.Equal(t, []domain.VATypeRule{newRule}, p.snapshotVATypes())
	repo.AssertExpectations(t)
}

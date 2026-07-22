package model

import (
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestListArchivedOperationalAssetsPageFiltersBySourceAndSearch(t *testing.T) {
	originalDB := DB
	t.Cleanup(func() { DB = originalDB })

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "operational-cost.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&OperationalAsset{}))
	DB = db

	assets := []OperationalAsset{
		{SourceType: OperationalAssetTypeCPAAccount, SourceKey: "cpa:one", DisplayName: "first@example.com", State: OperationalAssetStateArchived, ArchiveReason: "Frozen upstream", ArchivedAt: 30},
		{SourceType: OperationalAssetTypeCPAAccount, SourceKey: "cpa:two", DisplayName: "second@example.com", State: OperationalAssetStateArchived, ArchiveReason: "Expired", ArchivedAt: 20},
		{SourceType: OperationalAssetTypeSub2APIAccount, SourceKey: "sub2api:3", DisplayName: "third@example.com", State: OperationalAssetStateArchived, CostNote: "Frozen upstream", ArchivedAt: 10},
	}
	require.NoError(t, db.Create(&assets).Error)

	page := &common.PageInfo{Page: 1, PageSize: 20}
	items, total, err := ListArchivedOperationalAssetsPage(page, OperationalAssetTypeCPAAccount, "FROZEN")

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "first@example.com", items[0].DisplayName)
}

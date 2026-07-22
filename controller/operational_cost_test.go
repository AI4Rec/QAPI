package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeOperationalArchiveItemsIncludesStoredOutputs(t *testing.T) {
	snapshotPath := filepath.Join(t.TempDir(), "status.json")
	require.NoError(t, os.WriteFile(snapshotPath, []byte(`{
  "account_outputs": [{"asset_key":"cpa:stored","cumulative_output_quota":1000000}]
}`), 0o600))
	t.Setenv("QAPI_CAPACITY_SNAPSHOT_PATH", snapshotPath)

	items := makeOperationalArchiveItems([]model.OperationalAsset{
		{ID: 1, SourceType: model.OperationalAssetTypeCPAAccount, SourceKey: "cpa:stored"},
		{ID: 2, SourceType: model.OperationalAssetTypeSub2APIAccount, SourceKey: "sub2api:2", Snapshot: `{"cumulative_output_usd":12.5}`},
		{ID: 3, SourceType: model.OperationalAssetTypeChannel, SourceKey: "channel:3"},
	})

	require.Len(t, items, 3)
	require.NotNil(t, items[0].CumulativeOutputUSD)
	assert.Equal(t, 2.0, *items[0].CumulativeOutputUSD)
	require.NotNil(t, items[1].CumulativeOutputUSD)
	assert.Equal(t, 12.5, *items[1].CumulativeOutputUSD)
	assert.Nil(t, items[2].CumulativeOutputUSD)
}

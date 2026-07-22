package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeComponentVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "v7.2.93", normalizeComponentVersion("7.2.93"))
	assert.Equal(t, "v0.1.162", normalizeComponentVersion(" v0.1.162 "))
	assert.Empty(t, normalizeComponentVersion(""))
}

func TestComponentVersionIsNewer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		latest  string
		current string
		want    bool
	}{
		{name: "new patch", latest: "v7.2.93", current: "v7.2.66", want: true},
		{name: "same version", latest: "v0.1.162", current: "v0.1.162", want: false},
		{name: "installed is newer", latest: "v0.1.161", current: "v0.1.162", want: false},
		{name: "release beats prerelease", latest: "v1.0.0", current: "v1.0.0-rc.1", want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, componentVersionIsNewer(test.latest, test.current))
		})
	}
}

func TestManagedComponentsAreExplicitlyAllowlisted(t *testing.T) {
	t.Parallel()

	assert.True(t, IsManagedComponent(ManagedComponentCLIProxyAPI))
	assert.True(t, IsManagedComponent(ManagedComponentSub2API))
	assert.False(t, IsManagedComponent("../arbitrary-service"))
}

func TestParseManagedComponentInspection(t *testing.T) {
	t.Parallel()

	inspection, err := parseManagedComponentInspection("current_version=v7.2.66\nrollback_available=true\n")
	require.NoError(t, err)
	assert.Equal(t, "v7.2.66", inspection.CurrentVersion)
	assert.True(t, inspection.RollbackAvailable)

	_, err = parseManagedComponentInspection("current_version=../../etc/passwd\nrollback_available=true\n")
	assert.ErrorContains(t, err, "invalid installed version")
}

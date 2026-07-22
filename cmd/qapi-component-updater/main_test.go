package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleaseAssetNameMatchesOfficialNaming(t *testing.T) {
	t.Parallel()

	cpaArch := runtime.GOARCH
	if cpaArch == "arm64" {
		cpaArch = "aarch64"
	}
	assert.Equal(
		t,
		"CLIProxyAPI_7.2.93_linux_"+cpaArch+".tar.gz",
		releaseAssetName(components["cliproxyapi"], "v7.2.93"),
	)

	sub2APIArch := "amd64"
	if runtime.GOARCH == "arm64" {
		sub2APIArch = "arm64"
	}
	assert.Equal(
		t,
		"sub2api_0.1.162_linux_"+sub2APIArch+".tar.gz",
		releaseAssetName(components["sub2api"], "v0.1.162"),
	)
}

func TestVerifyChecksum(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "release.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, []byte("official release"), 0o600))
	digest := sha256.Sum256([]byte("official release"))
	checksumsPath := filepath.Join(tempDir, "checksums.txt")
	require.NoError(t, os.WriteFile(
		checksumsPath,
		[]byte(fmt.Sprintf("%x  release.tar.gz\n", digest)),
		0o600,
	))

	assert.NoError(t, verifyChecksum(archivePath, checksumsPath, "release.tar.gz"))
	require.NoError(t, os.WriteFile(archivePath, []byte("tampered release"), 0o600))
	assert.ErrorContains(t, verifyChecksum(archivePath, checksumsPath, "release.tar.gz"), "verification failed")
}

func TestExtractBinarySelectsExpectedFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "release.tar.gz")
	archive, err := os.Create(archivePath)
	require.NoError(t, err)
	gzipWriter := gzip.NewWriter(archive)
	tarWriter := tar.NewWriter(gzipWriter)

	unrelated := []byte("documentation")
	require.NoError(t, tarWriter.WriteHeader(&tar.Header{
		Name: "README.md",
		Mode: 0o644,
		Size: int64(len(unrelated)),
	}))
	_, err = tarWriter.Write(unrelated)
	require.NoError(t, err)

	binary := []byte("component binary")
	require.NoError(t, tarWriter.WriteHeader(&tar.Header{
		Name: "dist/sub2api",
		Mode: 0o755,
		Size: int64(len(binary)),
	}))
	_, err = tarWriter.Write(binary)
	require.NoError(t, err)
	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())
	require.NoError(t, archive.Close())

	destination := filepath.Join(tempDir, "sub2api")
	require.NoError(t, extractBinary(archivePath, destination, "sub2api"))
	extracted, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, binary, extracted)
}

func TestValidatedLinkTargetRejectsOutsideReleaseDirectory(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	releasesDir := filepath.Join(tempDir, "releases")
	require.NoError(t, os.MkdirAll(filepath.Join(releasesDir, "v1.0.0"), 0o755))
	currentLink := filepath.Join(tempDir, "current")
	require.NoError(t, os.Symlink(filepath.Join(releasesDir, "v1.0.0"), currentLink))

	target, err := validatedLinkTarget(currentLink, releasesDir)
	require.NoError(t, err)
	expectedTarget, err := filepath.EvalSymlinks(filepath.Join(releasesDir, "v1.0.0"))
	require.NoError(t, err)
	assert.Equal(t, expectedTarget, target)

	require.NoError(t, os.Remove(currentLink))
	require.NoError(t, os.Symlink(tempDir, currentLink))
	_, err = validatedLinkTarget(currentLink, releasesDir)
	assert.ErrorContains(t, err, "outside")
}

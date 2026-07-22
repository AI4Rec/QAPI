package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type diskStorageWithoutBytes struct {
	*bytes.Reader
}

func (d *diskStorageWithoutBytes) Close() error           { return nil }
func (d *diskStorageWithoutBytes) Bytes() ([]byte, error) { panic("Bytes must not be called") }
func (d *diskStorageWithoutBytes) Size() int64            { return d.Reader.Size() }
func (d *diskStorageWithoutBytes) IsDisk() bool           { return true }

func TestGetModelFromJSONBodyStreamsDiskStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5","group":"default","input":"large payload"}`)
	storage := &diskStorageWithoutBytes{Reader: bytes.NewReader(body)}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.KeyBodyStorage, storage)

	request, err := getModelFromJSONBody(c)
	require.NoError(t, err)
	assert.Equal(t, "gpt-5", request.Model)
	assert.Equal(t, "default", request.Group)
	position, err := storage.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	assert.Zero(t, position)
}

func TestGetModelFromJSONBodyRejectsTrailingJSONOnDisk(t *testing.T) {
	body := []byte(`{"model":"gpt-5"} {"group":"default"}`)
	storage := &diskStorageWithoutBytes{Reader: bytes.NewReader(body)}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.KeyBodyStorage, storage)

	_, err := getModelFromJSONBody(c)
	require.Error(t, err)
	position, seekErr := storage.Seek(0, io.SeekCurrent)
	require.NoError(t, seekErr)
	assert.Zero(t, position)
}

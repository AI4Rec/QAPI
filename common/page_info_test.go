package common

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPageQueryNormalizesPaginationBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		query        string
		expectedPage int
		expectedSize int
	}{
		{name: "defaults", expectedPage: 1, expectedSize: ItemsPerPage},
		{name: "valid values", query: "?p=3&page_size=50", expectedPage: 3, expectedSize: 50},
		{name: "caps oversized page", query: "?p=2&page_size=1000", expectedPage: 2, expectedSize: 100},
		{name: "rejects negative page", query: "?p=-2&page_size=20", expectedPage: 1, expectedSize: 20},
		{name: "rejects negative page size", query: "?p=2&page_size=-1", expectedPage: 2, expectedSize: ItemsPerPage},
		{name: "uses positive legacy ps", query: "?page_size=-1&ps=30", expectedPage: 1, expectedSize: 30},
		{name: "ignores negative legacy values", query: "?ps=-1&size=-5", expectedPage: 1, expectedSize: ItemsPerPage},
		{name: "uses legacy token size", query: "?size=100", expectedPage: 1, expectedSize: 100},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/api/items"+test.query, nil)
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = request

			pageInfo := GetPageQuery(context)
			require.NotNil(t, pageInfo)
			assert.Equal(t, test.expectedPage, pageInfo.Page)
			assert.Equal(t, test.expectedSize, pageInfo.PageSize)
		})
	}
}

package reward

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRewardCostBoundsMatchRedemption(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, item := range []struct {
		value string
		valid bool
	}{
		{"1", true}, {"1500000000", true}, {"2147483647", true},
		{"2147483648", false}, {"1.5", false}, {"0", false}, {"-1", false},
	} {
		t.Run(item.value, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/rewards", strings.NewReader(`{"name":"Reward","pointsCost":`+item.value+`}`))
			c.Request.Header.Set("Content-Type", "application/json")
			var req RewardReq
			err := c.ShouldBindJSON(&req)
			require.Equal(t, item.valid, err == nil, "%v", err)
		})
	}
}

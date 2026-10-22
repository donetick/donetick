package chore

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestXPRequestOmittedNullAndValue(t *testing.T) {
	var omitted ChoreReq
	require.NoError(t, json.Unmarshal([]byte(`{}`), &omitted))
	require.False(t, omitted.XPCategoryID.Present)
	var cleared ChoreReq
	require.NoError(t, json.Unmarshal([]byte(`{"xpCategoryId":null}`), &cleared))
	require.True(t, cleared.XPCategoryID.Present)
	require.Nil(t, cleared.XPCategoryID.Value)
	var chosen ChoreReq
	require.NoError(t, json.Unmarshal([]byte(`{"xpCategoryId":7}`), &chosen))
	require.True(t, chosen.XPCategoryID.Present)
	require.Equal(t, 7, *chosen.XPCategoryID.Value)
	for _, body := range []string{`{"xpCategoryId":1.5}`, `{"xpCategoryId":"7"}`, `{"xpCategoryId":{}}`} {
		var invalid ChoreReq
		require.Error(t, json.Unmarshal([]byte(body), &invalid))
	}
}

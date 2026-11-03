package chore

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chModel "donetick.com/core/internal/chore/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBulkCreateChores(t *testing.T) {
	router, db := newCreateChoreTestRouter(t)
	body := `{"chores":[
		{"name":"first","frequencyType":"once","assignStrategy":"random"},
		{"name":"invalid assignee","frequencyType":"once","assignStrategy":"random","assignees":[{"userId":999}]},
		{"name":"last","frequencyType":"daily","assignStrategy":"no_assignee","isPrivate":true}
	]}`
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/chores/bulk", strings.NewReader(body)))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var response struct {
		Res []struct {
			Index    int      `json:"index"`
			Status   int      `json:"status"`
			Res      int      `json:"res"`
			Warnings []string `json:"warnings"`
			Error    string   `json:"error"`
		} `json:"res"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Res, 3)
	for i, result := range response.Res {
		require.Equal(t, i, result.Index)
	}
	require.Equal(t, 200, response.Res[0].Status)
	require.NotEmpty(t, response.Res[0].Warnings)
	require.Equal(t, 400, response.Res[1].Status)
	require.Equal(t, "Assignee not found in circle", response.Res[1].Error)
	require.Equal(t, 200, response.Res[2].Status)

	var chores []chModel.Chore
	require.NoError(t, db.Order("id").Find(&chores).Error)
	require.Len(t, chores, 2)
	require.Equal(t, response.Res[0].Res, chores[0].ID)
	require.Equal(t, response.Res[2].Res, chores[1].ID)
	require.True(t, chores[1].IsPrivate)
	for _, chore := range chores {
		require.Equal(t, 1, chore.CreatedBy)
		require.Equal(t, 1, chore.CircleID)
		require.Equal(t, 1, chore.Frequency)
		require.True(t, chore.IsActive)
	}
}

func TestBulkCreateChoresRejectsInvalidRequestBeforeCreating(t *testing.T) {
	router, db := newCreateChoreTestRouter(t)
	valid := `{"name":"valid","frequencyType":"once","assignStrategy":"random"}`
	tooMany := make([]string, 101)
	for i := range tooMany {
		tooMany[i] = valid
	}
	for _, body := range []string{
		`{`, `{}`, `{"chores":null}`, `{"chores":[]}`,
		`{"chores":{}}`, `{"chores":[` + valid + `,`,
		`{"chores":[` + strings.Join(tooMany, ",") + `]}`,
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/chores/bulk", strings.NewReader(body)))
		require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	var count int64
	require.NoError(t, db.Model(&chModel.Chore{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestBulkCreateChoresContinuesAfterItemValidationFailures(t *testing.T) {
	for _, invalid := range []string{
		`{}`,
		`null`,
		`123`,
		`{"name":123}`,
		`{"name":"invalid frequency","frequencyType":"invalid","assignStrategy":"random"}`,
		`{"name":"invalid date","frequencyType":"once","assignStrategy":"random","nextDueDate":"not-a-date"}`,
	} {
		t.Run(invalid, func(t *testing.T) {
			router, db := newCreateChoreTestRouter(t)
			valid := `{"name":"valid","frequencyType":"once","assignStrategy":"random"}`
			body := `{"chores":[` + valid + `,` + invalid + `,` + valid + `]}`
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/chores/bulk", strings.NewReader(body)))
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			var response struct {
				Res []struct {
					Index   int    `json:"index"`
					Status  int    `json:"status"`
					Res     int    `json:"res"`
					Error   string `json:"error"`
					Details string `json:"details"`
				} `json:"res"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			require.Len(t, response.Res, 3)
			for i, result := range response.Res {
				require.Equal(t, i, result.Index)
			}
			require.Equal(t, http.StatusOK, response.Res[0].Status)
			require.Positive(t, response.Res[0].Res)
			require.Equal(t, http.StatusBadRequest, response.Res[1].Status)
			require.Equal(t, "Invalid request format", response.Res[1].Error)
			require.NotEmpty(t, response.Res[1].Details)
			require.Equal(t, http.StatusOK, response.Res[2].Status)
			require.Positive(t, response.Res[2].Res)
			var count int64
			require.NoError(t, db.Model(&chModel.Chore{}).Count(&count).Error)
			require.EqualValues(t, 2, count)
		})
	}
}

func TestBulkCreateChoresRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &Handler{}
	router.POST("/chores/bulk", h.BulkCreateChores)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/chores/bulk", bytes.NewBufferString(`{}`)))
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

package pushbullet

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"donetick.com/core/config"
	nModel "donetick.com/core/internal/notifier/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHTTPClient implements HTTPClient for testing.
type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func TestNewPushbulletNotifier_EmptyToken(t *testing.T) {
	cfg := &config.Config{
		Pushbullet: config.PushbulletConfig{APIToken: ""},
	}
	notifier := NewPushbulletNotifier(cfg)
	assert.Nil(t, notifier, "should return nil when API token is empty")
}

func TestNewPushbulletNotifier_WithToken(t *testing.T) {
	cfg := &config.Config{
		Pushbullet: config.PushbulletConfig{APIToken: "test-token"},
	}
	notifier := NewPushbulletNotifier(cfg)
	assert.NotNil(t, notifier)
	assert.Equal(t, "test-token", notifier.apiToken)
}

func TestSendNotification_EmptyTargetID(t *testing.T) {
	p := NewPushbulletNotifierWithClient("token", nil)
	notification := &nModel.NotificationDetails{
		Notification: nModel.Notification{
			TargetID: "",
			Text:     "hello",
		},
	}
	err := p.SendNotification(context.Background(), notification)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "targetID is empty")
}

func TestSendNotification_Success(t *testing.T) {
	var capturedReq *http.Request
	var capturedBody []byte

	client := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			capturedReq = req
			capturedBody, _ = io.ReadAll(req.Body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"iden":"abc"}`)),
			}, nil
		},
	}

	p := NewPushbulletNotifierWithClient("my-api-token", client)
	notification := &nModel.NotificationDetails{
		Notification: nModel.Notification{
			TargetID: "device-id-123",
			Text:     "Task is due!",
		},
	}

	err := p.SendNotification(context.Background(), notification)
	require.NoError(t, err)

	// Verify request was constructed correctly
	assert.Equal(t, http.MethodPost, capturedReq.Method)
	assert.Equal(t, pushbulletAPIURL, capturedReq.URL.String())
	assert.Equal(t, "my-api-token", capturedReq.Header.Get("Access-Token"))
	assert.Equal(t, "application/json", capturedReq.Header.Get("Content-Type"))

	// Verify payload
	assert.Contains(t, string(capturedBody), `"type":"note"`)
	assert.Contains(t, string(capturedBody), `"title":"Donetick"`)
	assert.Contains(t, string(capturedBody), `"body":"Task is due!"`)
}

func TestSendNotification_APIError(t *testing.T) {
	client := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(bytes.NewBufferString(`{"error":{"message":"Invalid access token"}}`)),
			}, nil
		},
	}

	p := NewPushbulletNotifierWithClient("bad-token", client)
	notification := &nModel.NotificationDetails{
		Notification: nModel.Notification{
			TargetID: "device-id",
			Text:     "hello",
		},
	}

	err := p.SendNotification(context.Background(), notification)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestSendNotification_NetworkError(t *testing.T) {
	client := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return nil, assert.AnError
		},
	}

	p := NewPushbulletNotifierWithClient("token", client)
	notification := &nModel.NotificationDetails{
		Notification: nModel.Notification{
			TargetID: "device-id",
			Text:     "hello",
		},
	}

	err := p.SendNotification(context.Background(), notification)
	require.Error(t, err)
}

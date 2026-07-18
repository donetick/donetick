package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"donetick.com/core/config"
)

// SMTP2GoSender sends transactional email via the SMTP2Go HTTP API instead
// of SMTP. Used when email.provider: smtp2go is set (e.g. donetick.com prod),
// where template IDs are pre-created in the SMTP2Go dashboard.
type SMTP2GoSender struct {
	apiKey           string
	fromMail         string
	appHost          string
	verifyTemplateID string
	resetTemplateID  string
}

const smtp2goEndpoint = "https://api.smtp2go.com/v3/email/send"

type smtp2goTemplateRequest struct {
	Sender       string      `json:"sender"`
	To           []string    `json:"to"`
	Subject      string      `json:"subject"`
	TemplateID   string      `json:"template_id"`
	TemplateData interface{} `json:"template_data"`
}

type smtp2goResponse struct {
	Data   interface{} `json:"data"`
	Error  interface{} `json:"error"`
	Result string      `json:"result"`
}

func NewSMTP2GoSender(conf *config.Config) *SMTP2GoSender {
	return &SMTP2GoSender{
		apiKey:           conf.EmailConfig.Key,
		fromMail:         conf.EmailConfig.Email,
		appHost:          conf.EmailConfig.AppHost,
		verifyTemplateID: conf.EmailConfig.VerifyTemplateID,
		resetTemplateID:  conf.EmailConfig.ResetPasswordTemplateID,
	}
}

func (s *SMTP2GoSender) sendTemplate(to, subject, templateID string, templateData interface{}) error {
	requestBody := smtp2goTemplateRequest{
		Sender:       s.fromMail,
		To:           []string{to},
		Subject:      subject,
		TemplateID:   templateID,
		TemplateData: templateData,
	}
	jsonBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", smtp2goEndpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Smtp2go-Api-Key", s.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("smtp2go API returned status: %s", resp.Status)
	}

	var apiResp smtp2goResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("failed to decode smtp2go response: %w", err)
	}

	if dataMap, ok := apiResp.Data.(map[string]interface{}); ok {
		if succeeded, exists := dataMap["succeeded"]; exists {
			if succeededCount, ok := succeeded.(float64); ok && succeededCount > 0 {
				return nil
			}
		}
	}
	if apiResp.Result != "success" && apiResp.Result != "" {
		return fmt.Errorf("smtp2go API result: %s, error: %v, data: %v", apiResp.Result, apiResp.Error, apiResp.Data)
	}
	return fmt.Errorf("smtp2go failed to send email, data: %v", apiResp.Data)
}

func (s *SMTP2GoSender) SendResetPasswordEmail(c context.Context, to, code string) error {
	fullURL := fmt.Sprintf("%s/password/update?c=%s", s.appHost, encodeEmailAndCode(to, code))
	templateData := map[string]interface{}{"verifyURL": fullURL}
	return s.sendTemplate(to, "Donetick! Password Reset", s.resetTemplateID, templateData)
}

func (s *SMTP2GoSender) SendVerificationEmail(to, code string) error {
	fullURL := fmt.Sprintf("%s/verify?c=%s", s.appHost, encodeEmailAndCode(to, code))
	templateData := map[string]interface{}{"verifyURL": fullURL}
	return s.sendTemplate(to, "Donetick! Verify your email", s.verifyTemplateID, templateData)
}

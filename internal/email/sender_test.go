package email

import (
	"testing"

	"donetick.com/core/config"
)

func TestNewEmailSender_WithUsername(t *testing.T) {
	// Test with separate username and email
	conf := &config.Config{
		EmailConfig: config.EmailConfig{
			Email:    "noreply@example.com",
			Username: "smtp_user_12345",
			Key:      "password",
			Host:     "smtp.example.com",
			Port:     587,
			AppHost:  "https://app.example.com",
		},
	}

	sender := NewEmailSender(conf)

	// Verify SMTP username is set to the provided username
	if sender.client.Username != "smtp_user_12345" {
		t.Errorf("Expected SMTP username to be 'smtp_user_12345', got '%s'", sender.client.Username)
	}

	// Verify fromEmail is set to the email address
	if sender.fromEmail != "noreply@example.com" {
		t.Errorf("Expected fromEmail to be 'noreply@example.com', got '%s'", sender.fromEmail)
	}

	// Verify appHost is set correctly
	if sender.appHost != "https://app.example.com" {
		t.Errorf("Expected appHost to be 'https://app.example.com', got '%s'", sender.appHost)
	}
}

func TestNewEmailSender_WithoutUsername(t *testing.T) {
	// Test backwards compatibility - when username is not provided
	conf := &config.Config{
		EmailConfig: config.EmailConfig{
			Email:   "user@example.com",
			Key:     "password",
			Host:    "smtp.example.com",
			Port:    587,
			AppHost: "https://app.example.com",
		},
	}

	sender := NewEmailSender(conf)

	// Verify SMTP username falls back to email
	if sender.client.Username != "user@example.com" {
		t.Errorf("Expected SMTP username to be 'user@example.com', got '%s'", sender.client.Username)
	}

	// Verify fromEmail is set to the email address
	if sender.fromEmail != "user@example.com" {
		t.Errorf("Expected fromEmail to be 'user@example.com', got '%s'", sender.fromEmail)
	}

	// Verify appHost is set correctly
	if sender.appHost != "https://app.example.com" {
		t.Errorf("Expected appHost to be 'https://app.example.com', got '%s'", sender.appHost)
	}
}

func TestNewEmailSender_WithEmptyUsername(t *testing.T) {
	// Test backwards compatibility - when username is empty string
	conf := &config.Config{
		EmailConfig: config.EmailConfig{
			Email:    "user@example.com",
			Username: "", // Explicitly empty
			Key:      "password",
			Host:     "smtp.example.com",
			Port:     587,
			AppHost:  "https://app.example.com",
		},
	}

	sender := NewEmailSender(conf)

	// Verify SMTP username falls back to email
	if sender.client.Username != "user@example.com" {
		t.Errorf("Expected SMTP username to be 'user@example.com', got '%s'", sender.client.Username)
	}

	// Verify fromEmail is set to the email address
	if sender.fromEmail != "user@example.com" {
		t.Errorf("Expected fromEmail to be 'user@example.com', got '%s'", sender.fromEmail)
	}
}

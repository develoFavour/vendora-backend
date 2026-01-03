package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

type BrevoEmailRequest struct {
	Sender      BrevoSender    `json:"sender"`
	To          []BrevoContact `json:"to"`
	Subject     string         `json:"subject"`
	HtmlContent string         `json:"htmlContent"`
}

type BrevoSender struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type BrevoContact struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

func SendEmail(to, subject, body string) error {
	apiKey := os.Getenv("BREVO_API_KEY")
	senderEmail := os.Getenv("SENDER_EMAIL")
	senderName := os.Getenv("SENDER_NAME")

	logrus.WithField("apiKey", apiKey[:10]+"...").Info("Using API key")
	logrus.WithField("senderEmail", senderEmail).Info("Using sender email")

	if apiKey == "" || senderEmail == "" {
		return fmt.Errorf("BREVO_API_KEY or SENDER_EMAIL not set")
	}

	// Prepare request payload
	payload := BrevoEmailRequest{
		Sender: BrevoSender{
			Name:  senderName,
			Email: senderEmail,
		},
		To: []BrevoContact{
			{Email: to},
		},
		Subject:     subject,
		HtmlContent: body,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	// Send HTTP request to Brevo API
	req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("brevo API returned status %d", resp.StatusCode)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("brevo API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
func IsDisposableEmail(email string) bool {
	disposableDomains := []string{
		"10minutemail.com", "tempmail.org", "guerrillamail.com",
		"mailinator.com", "yopmail.com", "temp-mail.org",
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return true
	}

	domain := strings.ToLower(parts[1])
	for _, disposable := range disposableDomains {
		if strings.Contains(domain, disposable) {
			return true
		}
	}
	return false
}

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/google/generative-ai-go/genai"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

type VerificationService struct {
	client *genai.Client
}

func NewVerificationService() (*VerificationService, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &VerificationService{client: client}, nil
}

func (s *VerificationService) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

// downloadImage fetches an image from a URL and returns its bytes.
func downloadImage(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status fetching image: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	return data, nil
}

type IdentityAnalysisResult struct {
	IsMatch         bool   `json:"isMatch"`
	Confidence      int    `json:"confidence"`
	ExtractedName   string `json:"extractedName"`
	RejectionReason string `json:"rejectionReason"`
}

// AnalyzeIdentity downloads the ID and Selfie, and asks Gemini to verify them against the user's name.
func (s *VerificationService) AnalyzeIdentity(ctx context.Context, idUrl, selfieUrl, expectedName string) (*IdentityAnalysisResult, error) {
	logrus.Infof("Starting AI Identity Analysis for user: %s", expectedName)

	idBytes, err := downloadImage(idUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to download ID image: %w", err)
	}

	selfieBytes, err := downloadImage(selfieUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to download selfie image: %w", err)
	}

	model := s.client.GenerativeModel("gemini-1.5-flash")
	model.ResponseMIMEType = "application/json"

	prompt := fmt.Sprintf(`You are an expert KYC (Know Your Customer) compliance officer.
You are tasked with verifying a user's identity. 
The expected name of the user is "%s".

You have been provided with two images:
1. An official ID document.
2. A selfie of the user.

Perform the following tasks:
1. Extract the full name visible on the ID document.
2. Determine if the extracted name strongly matches the expected name "%s" (ignoring minor typos or middle names).
3. Compare the face on the ID document with the face in the selfie. Determine if they are the same person.
4. Calculate a confidence score from 0 to 100 on how sure you are that this is a valid verification.

Return ONLY a JSON object exactly matching this schema:
{
  "isMatch": boolean (true if the name matches AND the faces match),
  "confidence": integer (0-100),
  "extractedName": string (the name found on the ID),
  "rejectionReason": string (if isMatch is false, explain why. e.g., "Name mismatch", "Faces do not match", "ID is unreadable", "Selfie is too blurry". Leave empty if isMatch is true)
}`, expectedName, expectedName)

	resp, err := model.GenerateContent(ctx,
		genai.ImageData("image/jpeg", idBytes), // Note: Gemini accepts jpeg/png data, content type is a hint
		genai.ImageData("image/jpeg", selfieBytes),
		genai.Text(prompt),
	)

	if err != nil {
		logrus.WithError(err).Error("Failed to trigger Gemini API")
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	// Extract the text response
	var jsonString string
	part := resp.Candidates[0].Content.Parts[0]
	if txt, ok := part.(genai.Text); ok {
		jsonString = string(txt)
	} else {
		return nil, fmt.Errorf("unexpected response format from Gemini")
	}

	logrus.Debugf("Gemini Response: %s", jsonString)

	var result IdentityAnalysisResult
	if err := json.Unmarshal([]byte(jsonString), &result); err != nil {
		logrus.WithError(err).Error("Failed to parse Gemini JSON response")
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return &result, nil
}

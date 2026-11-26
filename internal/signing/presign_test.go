package signing

import (
	"net/url"
	"testing"
	"time"
)

func TestPresignedURLGenerateAndVerify(t *testing.T) {
	secret := "super-secret-signing-key-2026"
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

	res, err := GeneratePresignedURL(secret, PresignRequest{
		BaseURL:   "http://localhost:8095",
		Method:    "GET",
		Bucket:    "models-prod",
		Key:       "weights/llama-v1.bin",
		VersionID: "v_12345",
		TTL:       10 * time.Minute,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("GeneratePresignedURL failed: %v", err)
	}

	parsed, err := url.Parse(res.URL)
	if err != nil {
		t.Fatalf("parse generated URL: %v", err)
	}

	// Verify within validity window
	if err := VerifyPresignedURL(secret, "GET", "models-prod", "weights/llama-v1.bin", parsed.Query(), now.Add(5*time.Minute)); err != nil {
		t.Fatalf("VerifyPresignedURL failed within TTL: %v", err)
	}

	// Reject wrong HTTP method (e.g. PUT using a GET token)
	if err := VerifyPresignedURL(secret, "PUT", "models-prod", "weights/llama-v1.bin", parsed.Query(), now.Add(1*time.Minute)); err != ErrInvalidMethod {
		t.Fatalf("expected ErrInvalidMethod when using GET token for PUT, got %v", err)
	}

	// Reject wrong object key
	if err := VerifyPresignedURL(secret, "GET", "models-prod", "weights/other.bin", parsed.Query(), now.Add(1*time.Minute)); err != ErrInvalidSignature {
		t.Fatalf("expected ErrInvalidSignature for wrong key, got %v", err)
	}

	// Reject expired token
	if err := VerifyPresignedURL(secret, "GET", "models-prod", "weights/llama-v1.bin", parsed.Query(), now.Add(11*time.Minute)); err != ErrSignatureExpired {
		t.Fatalf("expected ErrSignatureExpired after TTL, got %v", err)
	}

	// Reject tampered versionId parameter
	tamperedQ := parsed.Query()
	tamperedQ.Set("versionId", "v_99999")
	if err := VerifyPresignedURL(secret, "GET", "models-prod", "weights/llama-v1.bin", tamperedQ, now.Add(1*time.Minute)); err != ErrInvalidSignature {
		t.Fatalf("expected ErrInvalidSignature for tampered versionId, got %v", err)
	}
}

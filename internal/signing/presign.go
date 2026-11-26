package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrMissingSignature = errors.New("missing presigned url parameters")
	ErrInvalidMethod    = errors.New("invalid or mismatched HTTP method for presigned url")
	ErrSignatureExpired = errors.New("presigned url has expired")
	ErrInvalidSignature = errors.New("presigned url HMAC-SHA256 signature verification failed")
)

// PresignRequest defines the parameters required to generate an S3-style presigned URL.
type PresignRequest struct {
	BaseURL    string        `json:"base_url"`
	Method     string        `json:"method"`
	Bucket     string        `json:"bucket"`
	Key        string        `json:"key"`
	VersionID  string        `json:"version_id,omitempty"`
	TTL        time.Duration `json:"-"`
	TTLSeconds int64         `json:"ttl_seconds"`
	Now        time.Time     `json:"-"`
}

// PresignResult holds the generated presigned URL and its metadata.
type PresignResult struct {
	URL       string    `json:"url"`
	Path      string    `json:"path"`
	Method    string    `json:"method"`
	Bucket    string    `json:"bucket"`
	Key       string    `json:"key"`
	VersionID string    `json:"version_id,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	Signature string    `json:"signature"`
}

// Signer handles HMAC-SHA256 presigned URL creation and verification.
type Signer struct {
	secret []byte
}

// NewSigner creates a new Signer with the given secret key.
func NewSigner(secret string) *Signer {
	if strings.TrimSpace(secret) == "" {
		secret = "nimbusstore-default-signing-secret-key"
	}
	return &Signer{secret: []byte(secret)}
}

// GeneratePresignedURL builds an expiring HMAC-SHA256 presigned URL for GET or PUT object access.
func GeneratePresignedURL(secret string, req PresignRequest) (PresignResult, error) {
	return NewSigner(secret).Generate(req)
}

// VerifyPresignedURL validates a presigned URL's query parameters against method, bucket, key, and current time.
func VerifyPresignedURL(secret, method, bucket, key string, query url.Values, now time.Time) error {
	return NewSigner(secret).Verify(method, bucket, key, query, now)
}

func normalizeMethod(m string) (string, error) {
	upper := strings.ToUpper(strings.TrimSpace(m))
	if upper != "GET" && upper != "PUT" {
		return "", ErrInvalidMethod
	}
	return upper, nil
}

func canonicalPayload(method, bucket, key, versionID string, expiresUnix int64) string {
	return fmt.Sprintf("NIMBUS-HMAC-SHA256\n%s\n%s\n%s\n%s\n%d",
		method,
		strings.TrimSpace(bucket),
		strings.TrimPrefix(strings.TrimSpace(key), "/"),
		strings.TrimSpace(versionID),
		expiresUnix,
	)
}

func (s *Signer) computeMAC(method, bucket, key, versionID string, expiresUnix int64) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(canonicalPayload(method, bucket, key, versionID, expiresUnix)))
	return hex.EncodeToString(mac.Sum(nil))
}

// Generate creates a signed URL bound to method, bucket, key, optional versionId, and expiration.
func (s *Signer) Generate(req PresignRequest) (PresignResult, error) {
	method, err := normalizeMethod(req.Method)
	if err != nil {
		return PresignResult{}, err
	}
	bucket := strings.TrimSpace(req.Bucket)
	key := strings.TrimPrefix(strings.TrimSpace(req.Key), "/")
	if bucket == "" || key == "" {
		return PresignResult{}, errors.New("bucket and key are required")
	}

	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ttl := req.TTL
	if ttl <= 0 {
		if req.TTLSeconds > 0 {
			ttl = time.Duration(req.TTLSeconds) * time.Second
		} else {
			ttl = 15 * time.Minute
		}
	}
	if ttl > 7*24*time.Hour {
		ttl = 7 * 24 * time.Hour
	}

	expiresAt := now.Add(ttl).UTC()
	expiresUnix := expiresAt.Unix()
	sig := s.computeMAC(method, bucket, key, req.VersionID, expiresUnix)

	q := url.Values{}
	q.Set("X-Nimbus-Method", method)
	q.Set("X-Nimbus-Expires", strconv.FormatInt(expiresUnix, 10))
	if req.VersionID != "" {
		q.Set("versionId", req.VersionID)
	}
	q.Set("X-Nimbus-Signature", sig)

	escapedKeyParts := strings.Split(key, "/")
	for i, p := range escapedKeyParts {
		escapedKeyParts[i] = url.PathEscape(p)
	}
	relPath := fmt.Sprintf("/s3/%s/%s?%s", url.PathEscape(bucket), strings.Join(escapedKeyParts, "/"), q.Encode())
	fullURL := relPath
	if strings.TrimSpace(req.BaseURL) != "" {
		fullURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/") + relPath
	}

	return PresignResult{
		URL:       fullURL,
		Path:      relPath,
		Method:    method,
		Bucket:    bucket,
		Key:       key,
		VersionID: req.VersionID,
		ExpiresAt: expiresAt,
		Signature: sig,
	}, nil
}

// Verify checks the HMAC-SHA256 signature in constant time and enforces expiration & method binding.
func (s *Signer) Verify(method, bucket, key string, query url.Values, now time.Time) error {
	reqMethod, err := normalizeMethod(method)
	if err != nil {
		return err
	}
	signedMethod := strings.ToUpper(strings.TrimSpace(query.Get("X-Nimbus-Method")))
	expiresStr := strings.TrimSpace(query.Get("X-Nimbus-Expires"))
	sigHex := strings.ToLower(strings.TrimSpace(query.Get("X-Nimbus-Signature")))
	versionID := strings.TrimSpace(query.Get("versionId"))

	if signedMethod == "" || expiresStr == "" || sigHex == "" {
		return ErrMissingSignature
	}
	if signedMethod != reqMethod {
		return ErrInvalidMethod
	}

	expiresUnix, parseErr := strconv.ParseInt(expiresStr, 10, 64)
	if parseErr != nil {
		return ErrInvalidSignature
	}

	if now.IsZero() {
		now = time.Now().UTC()
	}
	if now.UTC().Unix() > expiresUnix {
		return ErrSignatureExpired
	}

	expectedHex := s.computeMAC(signedMethod, bucket, key, versionID, expiresUnix)
	providedBytes, err1 := hex.DecodeString(sigHex)
	expectedBytes, err2 := hex.DecodeString(expectedHex)
	if err1 != nil || err2 != nil {
		return ErrInvalidSignature
	}
	if !hmac.Equal(providedBytes, expectedBytes) {
		return ErrInvalidSignature
	}
	return nil
}

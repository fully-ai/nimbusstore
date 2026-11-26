package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fully-ai/nimbusstore/internal/cas"
	"github.com/fully-ai/nimbusstore/internal/signing"
	"github.com/fully-ai/nimbusstore/internal/storage"
	"github.com/fully-ai/nimbusstore/internal/web"
)

// ServerConfig holds runtime configuration for the NimbusStore HTTP server.
type ServerConfig struct {
	AccessKey     string
	SecretKey     string
	ApiToken      string
	SigningSecret string
	RateLimitRPS  float64
	RateBurst     float64
}

type visitorBucket struct {
	tokens   float64
	lastSeen time.Time
}

// Server exposes the NimbusStore REST API, S3-compatible data plane, and Governance Web UI.
type Server struct {
	cfg     ServerConfig
	cas     *cas.Engine
	catalog *storage.Catalog
	signer  *signing.Signer
	mux     *http.ServeMux

	rateMu   sync.Mutex
	visitors map[string]*visitorBucket
	started  time.Time
}

// NewServer constructs a configured NimbusStore HTTP Server.
func NewServer(cfg ServerConfig, casEngine *cas.Engine, catalog *storage.Catalog) *Server {
	if cfg.SigningSecret == "" {
		if cfg.SecretKey != "" {
			cfg.SigningSecret = cfg.SecretKey
		} else {
			cfg.SigningSecret = "nimbusstore-default-hmac-signing-secret"
		}
	}
	if cfg.RateLimitRPS <= 0 {
		cfg.RateLimitRPS = 50
	}
	if cfg.RateBurst <= 0 {
		cfg.RateBurst = 100
	}

	s := &Server{
		cfg:      cfg,
		cas:      casEngine,
		catalog:  catalog,
		signer:   signing.NewSigner(cfg.SigningSecret),
		mux:      http.NewServeMux(),
		visitors: make(map[string]*visitorBucket),
		started:  time.Now().UTC(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/", s.handleRoot)
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/metrics", s.handleMetrics)
	s.mux.HandleFunc("/api/v1/buckets", s.handleBuckets)
	s.mux.HandleFunc("/api/v1/buckets/", s.handleBucketSubroutes)
	s.mux.HandleFunc("/api/v1/presign", s.handlePresign)
	s.mux.HandleFunc("/api/v1/scrub", s.handleScrub)
	s.mux.HandleFunc("/api/v1/audits", s.handleAudits)
	s.mux.HandleFunc("/api/v1/admin/backup", s.handleAdminBackup)
	s.mux.HandleFunc("/api/v1/admin/restore", s.handleAdminRestore)
	s.mux.HandleFunc("/s3/", s.handleS3Object)
}

// ServeHTTP applies security headers, token-bucket rate limiting, and dispatches requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")

	if !s.allowRate(r) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error": "rate limit exceeded",
		})
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) allowRate(r *http.Request) bool {
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "" {
		ip = host
	}
	now := time.Now()

	s.rateMu.Lock()
	defer s.rateMu.Unlock()

	v, exists := s.visitors[ip]
	if !exists {
		s.visitors[ip] = &visitorBucket{
			tokens:   s.cfg.RateBurst - 1,
			lastSeen: now,
		}
		return true
	}

	elapsed := now.Sub(v.lastSeen).Seconds()
	v.lastSeen = now
	v.tokens += elapsed * s.cfg.RateLimitRPS
	if v.tokens > s.cfg.RateBurst {
		v.tokens = s.cfg.RateBurst
	}
	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}

func (s *Server) authRequired() bool {
	return s.cfg.ApiToken != "" || (s.cfg.AccessKey != "" && s.cfg.SecretKey != "")
}

func (s *Server) checkRequestAuth(r *http.Request) (string, bool) {
	if !s.authRequired() {
		return "console-user", true
	}

	// Check Bearer token
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") {
		provided := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if s.cfg.ApiToken != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(s.cfg.ApiToken)) == 1 {
			return "bearer-token", true
		}
		if s.cfg.SecretKey != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(s.cfg.SecretKey)) == 1 {
			return "secret-key", true
		}
	}

	// Check S3-style Access-Key / Secret-Key headers
	ak := strings.TrimSpace(r.Header.Get("X-Nimbus-Access-Key"))
	sk := strings.TrimSpace(r.Header.Get("X-Nimbus-Secret-Key"))
	if s.cfg.AccessKey != "" && s.cfg.SecretKey != "" {
		akMatch := subtle.ConstantTimeCompare([]byte(ak), []byte(s.cfg.AccessKey)) == 1
		skMatch := subtle.ConstantTimeCompare([]byte(sk), []byte(s.cfg.SecretKey)) == 1
		if akMatch && skMatch {
			return ak, true
		}
	}

	return "", false
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "route not found"})
		return
	}
	web.ServeDashboard(w, r)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	casStats := s.cas.Stats()
	healthy := true
	if casStats.LastScrub != nil && !casStats.LastScrub.Healthy {
		healthy = false
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"service":        "nimbusstore",
		"version":        "v0.1.0",
		"cas_healthy":    healthy,
		"uptime_seconds": int64(time.Since(s.started).Seconds()),
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	casStats := s.cas.Stats()
	catSum := s.catalog.Summary()

	var dedupRatio float64 = 1.0
	if casStats.PhysicalBytes > 0 && catSum.TotalLogicalBytes > casStats.PhysicalBytes {
		dedupRatio = float64(catSum.TotalLogicalBytes) / float64(casStats.PhysicalBytes)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"service":             "nimbusstore",
		"cas":                 casStats,
		"catalog":             catSum,
		"deduplication_ratio": dedupRatio,
	})
}

type bucketRequest struct {
	Name              string `json:"name"`
	QuotaMB           int64  `json:"quota_mb"`
	VersioningEnabled bool   `json:"versioning_enabled"`
	PublicRead        bool   `json:"public_read"`
	RetentionDays     int    `json:"retention_days"`
}

func (s *Server) handleBuckets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		buckets := s.catalog.ListBuckets()
		writeJSON(w, http.StatusOK, map[string]any{"buckets": buckets})
	case http.MethodPost:
		actor, ok := s.checkRequestAuth(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		var req bucketRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json payload"})
			return
		}
		// If bucket already exists, update its governance settings
		if _, getErr := s.catalog.GetBucket(req.Name); getErr == nil {
			b, err := s.catalog.UpdateBucket(req.Name, req.QuotaMB, req.VersioningEnabled, req.PublicRead, req.RetentionDays, actor)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"bucket": b, "updated": true})
			return
		}
		b, err := s.catalog.CreateBucket(req.Name, req.QuotaMB, req.VersioningEnabled, req.PublicRead, req.RetentionDays, actor)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"bucket": b})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

func (s *Server) handleBucketSubroutes(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/buckets/")
	parts := strings.SplitN(trimmed, "/", 2)
	bucketName := strings.TrimSpace(parts[0])
	if bucketName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bucket name required"})
		return
	}

	if len(parts) == 1 || parts[1] == "" {
		switch r.Method {
		case http.MethodGet:
			b, err := s.catalog.GetBucket(bucketName)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"bucket": b})
		case http.MethodDelete:
			actor, ok := s.checkRequestAuth(r)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			if err := s.catalog.DeleteBucket(bucketName, actor); err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"deleted": bucketName})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		}
		return
	}

	if parts[1] == "objects" {
		s.handleBucketObjects(w, r, bucketName)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]any{"error": "unknown bucket subroute"})
}

func parseTags(raw string) map[string]string {
	out := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return out
}

func (s *Server) handleBucketObjects(w http.ResponseWriter, r *http.Request, bucketName string) {
	switch r.Method {
	case http.MethodGet:
		prefix := r.URL.Query().Get("prefix")
		includeVersions := r.URL.Query().Get("versions") == "true"
		objs, err := s.catalog.ListObjects(bucketName, prefix, includeVersions)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"bucket":  bucketName,
			"count":   len(objs),
			"objects": objs,
		})

	case http.MethodPost:
		actor, ok := s.checkRequestAuth(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		key := strings.TrimSpace(r.URL.Query().Get("key"))
		if key == "" {
			key = strings.TrimSpace(r.Header.Get("X-Nimbus-Key"))
		}
		if key == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query parameter 'key' is required"})
			return
		}
		tags := parseTags(r.URL.Query().Get("tags"))
		for k, v := range r.Header {
			lower := strings.ToLower(k)
			if strings.HasPrefix(lower, "x-amz-meta-") && len(v) > 0 {
				tags[strings.TrimPrefix(lower, "x-amz-meta-")] = v[0]
			}
		}
		ver, err := s.ingestObject(bucketName, key, r.Header.Get("Content-Type"), r.Body, tags, actor)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"object": ver})

	case http.MethodDelete:
		actor, ok := s.checkRequestAuth(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		key := strings.TrimSpace(r.URL.Query().Get("key"))
		versionID := strings.TrimSpace(r.URL.Query().Get("versionId"))
		if key == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query parameter 'key' is required"})
			return
		}
		removed, err := s.catalog.DeleteObject(bucketName, key, versionID, actor)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": removed})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

func (s *Server) ingestObject(bucketName, key, contentType string, body io.Reader, tags map[string]string, actor string) (storage.ObjectVersion, error) {
	if _, err := s.catalog.GetBucket(bucketName); err != nil {
		return storage.ObjectVersion{}, err
	}
	putRes, err := s.cas.Put(body)
	if err != nil {
		return storage.ObjectVersion{}, err
	}
	ver, err := s.catalog.PutObjectVersion(bucketName, key, putRes.SHA256, putRes.Size, contentType, tags, putRes.Deduplicated, actor)
	if err != nil {
		if !putRes.Deduplicated {
			_ = s.cas.DeleteBlob(putRes.SHA256)
		}
		return storage.ObjectVersion{}, err
	}
	return ver, nil
}

// handleS3Object serves the S3-compatible data plane at /s3/{bucket}/{key...} supporting GET, HEAD, PUT, DELETE,
// presigned URL HMAC verification, bucket public_read policy, and versionId selection.
func (s *Server) handleS3Object(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/s3/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "s3 path must be /s3/{bucket}/{key}"})
		return
	}
	bucketName := strings.TrimSpace(parts[0])
	key := strings.TrimSpace(parts[1])

	b, err := s.catalog.GetBucket(bucketName)
	if err != nil {
		writeError(w, err)
		return
	}

	// Authenticate via Presigned URL query parameters, Public Read policy (for GET/HEAD), or API headers
	hasPresignSig := r.URL.Query().Get("X-Nimbus-Signature") != ""
	actor := "anonymous"
	if hasPresignSig {
		verifyMethod := r.Method
		if verifyMethod == http.MethodHead {
			verifyMethod = http.MethodGet
		}
		if err := s.signer.Verify(verifyMethod, bucketName, key, r.URL.Query(), time.Now().UTC()); err != nil {
			s.catalog.RecordAudit("S3_PRESIGN_REJECTED", bucketName, key, r.URL.Query().Get("versionId"), "presigned", "DENIED", err.Error())
			writeJSON(w, http.StatusForbidden, map[string]any{"error": err.Error()})
			return
		}
		actor = "presigned-url"
	} else {
		isRead := r.Method == http.MethodGet || r.Method == http.MethodHead
		if !(isRead && b.PublicRead) {
			authedActor, ok := s.checkRequestAuth(r)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authentication or valid presigned url required"})
				return
			}
			actor = authedActor
		} else {
			actor = "public-read"
		}
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		versionID := strings.TrimSpace(r.URL.Query().Get("versionId"))
		ver, err := s.catalog.GetObjectVersion(bucketName, key, versionID)
		if err != nil {
			writeError(w, err)
			return
		}
		rc, size, err := s.cas.Open(ver.SHA256)
		if err != nil {
			writeError(w, err)
			return
		}
		defer rc.Close()

		w.Header().Set("Content-Type", ver.ContentType)
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		w.Header().Set("ETag", fmt.Sprintf("%q", ver.SHA256))
		w.Header().Set("X-Nimbus-Version-Id", ver.VersionID)
		w.Header().Set("X-Nimbus-Content-SHA256", ver.SHA256)
		for tk, tv := range ver.Tags {
			w.Header().Set("X-Amz-Meta-"+tk, tv)
		}
		s.catalog.RecordAudit("GET_OBJECT", bucketName, key, ver.VersionID, actor, "SUCCESS", fmt.Sprintf("bytes=%d", size))

		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, rc)

	case http.MethodPut:
		tags := parseTags(r.URL.Query().Get("tags"))
		for k, v := range r.Header {
			lower := strings.ToLower(k)
			if strings.HasPrefix(lower, "x-amz-meta-") && len(v) > 0 {
				tags[strings.TrimPrefix(lower, "x-amz-meta-")] = v[0]
			}
		}
		ver, err := s.ingestObject(bucketName, key, r.Header.Get("Content-Type"), r.Body, tags, actor)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("ETag", fmt.Sprintf("%q", ver.SHA256))
		w.Header().Set("X-Nimbus-Version-Id", ver.VersionID)
		w.Header().Set("X-Nimbus-Deduplicated", strconv.FormatBool(ver.Deduplicated))
		writeJSON(w, http.StatusCreated, map[string]any{"object": ver})

	case http.MethodDelete:
		versionID := strings.TrimSpace(r.URL.Query().Get("versionId"))
		removed, err := s.catalog.DeleteObject(bucketName, key, versionID, actor)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": removed})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

func (s *Server) handlePresign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	actor, ok := s.checkRequestAuth(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	var req signing.PresignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json payload"})
		return
	}
	res, err := s.signer.Generate(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	s.catalog.RecordAudit("PRESIGN_URL", res.Bucket, res.Key, res.VersionID, actor, "SUCCESS",
		fmt.Sprintf("method=%s expires=%s", res.Method, res.ExpiresAt.Format(time.RFC3339)))
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleScrub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	actor, ok := s.checkRequestAuth(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	report, err := s.cas.ScrubAll()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	status := "SUCCESS"
	if !report.Healthy {
		status = "CORRUPTION_DETECTED"
	}
	s.catalog.RecordAudit("SCRUB_CAS", "", "", "", actor, status,
		fmt.Sprintf("verified=%d/%d corrupted=%d bytes=%d", report.VerifiedBlobs, report.TotalBlobs, report.CorruptedCount, report.TotalBytesVerified))
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleAudits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	audits := s.catalog.ListAudits(100)
	writeJSON(w, http.StatusOK, map[string]any{"audits": audits})
}

func (s *Server) handleAdminBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	actor, ok := s.checkRequestAuth(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	snap, err := s.catalog.ExportBackup(actor)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleAdminRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	actor, ok := s.checkRequestAuth(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	var snap storage.CatalogSnapshot
	if err := json.NewDecoder(r.Body).Decode(&snap); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid backup snapshot json"})
		return
	}
	if err := s.catalog.ImportRestore(snap, actor); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"restored": true, "buckets": len(snap.Buckets)})
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrBucketNotFound), errors.Is(err, storage.ErrObjectNotFound), errors.Is(err, cas.ErrBlobNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
	case errors.Is(err, storage.ErrBucketAlreadyExists):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
	case errors.Is(err, storage.ErrQuotaExceeded), errors.Is(err, cas.ErrPayloadTooBig):
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": err.Error()})
	case errors.Is(err, storage.ErrInvalidBucketName), errors.Is(err, storage.ErrInvalidObjectKey), errors.Is(err, cas.ErrInvalidHash):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

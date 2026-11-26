package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fully-ai/nimbusstore/internal/cas"
	"github.com/fully-ai/nimbusstore/internal/signing"
	"github.com/fully-ai/nimbusstore/internal/storage"
)

func setupTestServer(t *testing.T) (*Server, *cas.Engine, *storage.Catalog) {
	t.Helper()
	dir := t.TempDir()
	casEng, err := cas.NewEngine(filepath.Join(dir, "cas"), 10*1024*1024)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	cat, err := storage.NewCatalog(filepath.Join(dir, "meta"))
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	srv := NewServer(ServerConfig{
		ApiToken:      "test-admin-token",
		SigningSecret: "test-hmac-secret",
		RateLimitRPS:  500,
		RateBurst:     500,
	}, casEng, cat)
	return srv, casEng, cat
}

func doAuthReq(t *testing.T, srv http.Handler, method, path string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestDashboardAndHealthEndpoints(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "NimbusStore Object & Bucket Governance Console") {
		t.Fatalf("expected dashboard HTML on GET /, got status=%d", rec.Code)
	}

	hRec := httptest.NewRecorder()
	srv.ServeHTTP(hRec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if hRec.Code != http.StatusOK || !strings.Contains(hRec.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected /healthz response: %d %s", hRec.Code, hRec.Body.String())
	}
}

func TestBucketVersioningDeduplicationAndPresignedDownload(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	// 1. Create a versioned bucket
	createBody := `{"name":"artifacts","quota_mb":10,"versioning_enabled":true,"public_read":false,"retention_days":30}`
	rec := doAuthReq(t, srv, http.MethodPost, "/api/v1/buckets", strings.NewReader(createBody), "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create bucket status=%d body=%s", rec.Code, rec.Body.String())
	}

	// 2. Upload v1 and v2 of an object key
	recV1 := doAuthReq(t, srv, http.MethodPost, "/api/v1/buckets/artifacts/objects?key=releases/app.txt&tags=stage=beta", strings.NewReader("release-v1-payload"), "text/plain")
	if recV1.Code != http.StatusCreated {
		t.Fatalf("upload v1 failed: %d %s", recV1.Code, recV1.Body.String())
	}
	var v1Resp struct {
		Object storage.ObjectVersion `json:"object"`
	}
	_ = json.Unmarshal(recV1.Body.Bytes(), &v1Resp)

	recV2 := doAuthReq(t, srv, http.MethodPost, "/api/v1/buckets/artifacts/objects?key=releases/app.txt&tags=stage=ga", strings.NewReader("release-v2-payload"), "text/plain")
	if recV2.Code != http.StatusCreated {
		t.Fatalf("upload v2 failed: %d %s", recV2.Code, recV2.Body.String())
	}
	var v2Resp struct {
		Object storage.ObjectVersion `json:"object"`
	}
	_ = json.Unmarshal(recV2.Body.Bytes(), &v2Resp)

	// 3. Upload identical v2 payload to a different key via S3 PUT to verify CAS deduplication
	recDedup := doAuthReq(t, srv, http.MethodPut, "/s3/artifacts/releases/app-copy.txt", strings.NewReader("release-v2-payload"), "text/plain")
	if recDedup.Code != http.StatusCreated {
		t.Fatalf("s3 put failed: %d %s", recDedup.Code, recDedup.Body.String())
	}
	if recDedup.Header().Get("X-Nimbus-Deduplicated") != "true" {
		t.Fatalf("expected X-Nimbus-Deduplicated: true header on identical payload")
	}

	// 4. Download latest version vs specific v1 versionId
	getLatest := doAuthReq(t, srv, http.MethodGet, "/s3/artifacts/releases/app.txt", nil, "")
	if getLatest.Code != http.StatusOK || getLatest.Body.String() != "release-v2-payload" {
		t.Fatalf("expected latest payload release-v2-payload, got %d %q", getLatest.Code, getLatest.Body.String())
	}

	getV1 := doAuthReq(t, srv, http.MethodGet, "/s3/artifacts/releases/app.txt?versionId="+v1Resp.Object.VersionID, nil, "")
	if getV1.Code != http.StatusOK || getV1.Body.String() != "release-v1-payload" {
		t.Fatalf("expected historical v1 payload release-v1-payload, got %d %q", getV1.Code, getV1.Body.String())
	}

	// 5. Generate Presigned URL for v1 and download anonymously without Authorization header
	presignBody := `{"method":"GET","bucket":"artifacts","key":"releases/app.txt","version_id":"` + v1Resp.Object.VersionID + `","ttl_seconds":300}`
	presignRec := doAuthReq(t, srv, http.MethodPost, "/api/v1/presign", strings.NewReader(presignBody), "application/json")
	if presignRec.Code != http.StatusOK {
		t.Fatalf("presign failed: %d %s", presignRec.Code, presignRec.Body.String())
	}
	var presignRes signing.PresignResult
	_ = json.Unmarshal(presignRec.Body.Bytes(), &presignRes)

	anonReq := httptest.NewRequest(http.MethodGet, presignRes.Path, nil)
	anonRec := httptest.NewRecorder()
	srv.ServeHTTP(anonRec, anonReq)
	if anonRec.Code != http.StatusOK || anonRec.Body.String() != "release-v1-payload" {
		t.Fatalf("anonymous presigned GET failed: %d %q", anonRec.Code, anonRec.Body.String())
	}

	// Tampered presigned URL must be rejected with 403 Forbidden
	tamperedReq := httptest.NewRequest(http.MethodGet, presignRes.Path+"00", nil)
	tamperedRec := httptest.NewRecorder()
	srv.ServeHTTP(tamperedRec, tamperedReq)
	if tamperedRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for tampered presigned URL, got %d", tamperedRec.Code)
	}

	// 6. Delete object without versionId creates a Delete Marker; latest returns 404 while historical v1 still resolves
	delRec := doAuthReq(t, srv, http.MethodDelete, "/api/v1/buckets/artifacts/objects?key=releases/app.txt", nil, "")
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete object failed: %d %s", delRec.Code, delRec.Body.String())
	}
	getAfterDel := doAuthReq(t, srv, http.MethodGet, "/s3/artifacts/releases/app.txt", nil, "")
	if getAfterDel.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete marker on latest key, got %d", getAfterDel.Code)
	}
	getV1AfterDel := doAuthReq(t, srv, http.MethodGet, "/s3/artifacts/releases/app.txt?versionId="+v1Resp.Object.VersionID, nil, "")
	if getV1AfterDel.Code != http.StatusOK || getV1AfterDel.Body.String() != "release-v1-payload" {
		t.Fatalf("expected historical v1 to survive delete marker, got %d", getV1AfterDel.Code)
	}
}

func TestQuotaEnforcementScrubAndBackupRestore(t *testing.T) {
	srv, casEng, _ := setupTestServer(t)

	// Create bucket with 1 MB quota
	createBody := `{"name":"tiny-bucket","quota_mb":1,"versioning_enabled":false,"public_read":true,"retention_days":7}`
	rec := doAuthReq(t, srv, http.MethodPost, "/api/v1/buckets", strings.NewReader(createBody), "application/json")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create tiny-bucket failed: %d", rec.Code)
	}

	// Upload 600 KB payload -> succeeds
	payload600KB := bytes.Repeat([]byte("A"), 600*1024)
	up1 := doAuthReq(t, srv, http.MethodPut, "/s3/tiny-bucket/file1.bin", bytes.NewReader(payload600KB), "application/octet-stream")
	if up1.Code != http.StatusCreated {
		t.Fatalf("expected 600KB upload to succeed, got %d %s", up1.Code, up1.Body.String())
	}

	// Upload another 600 KB payload to different key -> exceeds 1 MB quota (1200 KB > 1024 KB) -> 413
	payload600KB2 := bytes.Repeat([]byte("B"), 600*1024)
	up2 := doAuthReq(t, srv, http.MethodPut, "/s3/tiny-bucket/file2.bin", bytes.NewReader(payload600KB2), "application/octet-stream")
	if up2.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 quota exceeded, got %d %s", up2.Code, up2.Body.String())
	}

	// Run bit-rot scrub via API
	scrubRec := doAuthReq(t, srv, http.MethodPost, "/api/v1/scrub", nil, "")
	if scrubRec.Code != http.StatusOK {
		t.Fatalf("scrub failed: %d", scrubRec.Code)
	}
	var report cas.ScrubReport
	_ = json.Unmarshal(scrubRec.Body.Bytes(), &report)
	if !report.Healthy || report.VerifiedBlobs != 1 {
		t.Fatalf("expected 1 healthy verified blob, got %+v", report)
	}

	// Export backup snapshot, delete bucket, then restore backup and verify bucket returns
	backupRec := doAuthReq(t, srv, http.MethodGet, "/api/v1/admin/backup", nil, "")
	if backupRec.Code != http.StatusOK {
		t.Fatalf("backup failed: %d", backupRec.Code)
	}
	backupBytes := backupRec.Body.Bytes()

	delBucket := doAuthReq(t, srv, http.MethodDelete, "/api/v1/buckets/tiny-bucket", nil, "")
	if delBucket.Code != http.StatusOK {
		t.Fatalf("delete bucket failed: %d", delBucket.Code)
	}

	restoreRec := doAuthReq(t, srv, http.MethodPost, "/api/v1/admin/restore", bytes.NewReader(backupBytes), "application/json")
	if restoreRec.Code != http.StatusOK {
		t.Fatalf("restore failed: %d %s", restoreRec.Code, restoreRec.Body.String())
	}

	// Anonymous public-read download of restored object
	pubReq := httptest.NewRequest(http.MethodGet, "/s3/tiny-bucket/file1.bin", nil)
	pubRec := httptest.NewRecorder()
	srv.ServeHTTP(pubRec, pubReq)
	if pubRec.Code != http.StatusOK || pubRec.Body.Len() != len(payload600KB) {
		t.Fatalf("expected public read on restored bucket to return 600KB, got status=%d len=%d", pubRec.Code, pubRec.Body.Len())
	}
	_ = casEng
}

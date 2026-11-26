package cas

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidHash   = errors.New("invalid sha256 digest")
	ErrBlobNotFound  = errors.New("cas blob not found")
	ErrPayloadTooBig = errors.New("object exceeds maximum allowed payload size")
)

// PutResult captures the outcome of storing a payload in the CAS engine.
type PutResult struct {
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
	Deduplicated bool   `json:"deduplicated"`
}

// CorruptedBlob describes a blob that failed SHA-256 bit-rot verification.
type CorruptedBlob struct {
	ExpectedSHA256 string `json:"expected_sha256"`
	ActualSHA256   string `json:"actual_sha256"`
	Size           int64  `json:"size"`
	Error          string `json:"error,omitempty"`
}

// ScrubReport contains full telemetry from a bit-rot integrity scrub.
type ScrubReport struct {
	TotalBlobs         int             `json:"total_blobs"`
	VerifiedBlobs      int             `json:"verified_blobs"`
	CorruptedCount     int             `json:"corrupted_count"`
	CorruptedBlobs     []CorruptedBlob `json:"corrupted_blobs"`
	TotalBytesVerified int64           `json:"total_bytes_verified"`
	DurationMs         int64           `json:"duration_ms"`
	ScrubbedAt         time.Time       `json:"scrubbed_at"`
	Healthy            bool            `json:"healthy"`
}

// EngineStats summarizes physical CAS storage and deduplication telemetry.
type EngineStats struct {
	UniqueBlobs     int          `json:"unique_blobs"`
	PhysicalBytes   int64        `json:"physical_bytes"`
	TotalPuts       int64        `json:"total_puts"`
	DedupHits       int64        `json:"dedup_hits"`
	DedupBytesSaved int64        `json:"dedup_bytes_saved"`
	LastScrub       *ScrubReport `json:"last_scrub,omitempty"`
}

// Engine implements a thread-safe Content-Addressable Storage (CAS) blob store.
type Engine struct {
	mu              sync.RWMutex
	rootDir         string
	blobsDir        string
	tmpDir          string
	maxObjectBytes  int64
	totalPuts       int64
	dedupHits       int64
	dedupBytesSaved int64
	lastScrub       *ScrubReport
}

// NewEngine creates and initializes a CAS Engine rooted at rootDir.
func NewEngine(rootDir string, maxObjectBytes int64) (*Engine, error) {
	blobsDir := filepath.Join(rootDir, "blobs")
	tmpDir := filepath.Join(rootDir, "tmp")
	if err := os.MkdirAll(blobsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create blobs dir: %w", err)
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("create tmp dir: %w", err)
	}
	if maxObjectBytes <= 0 {
		maxObjectBytes = 64 * 1024 * 1024 // 64 MB default max object size
	}
	return &Engine{
		rootDir:        rootDir,
		blobsDir:       blobsDir,
		tmpDir:         tmpDir,
		maxObjectBytes: maxObjectBytes,
	}, nil
}

// ValidateHash checks that h is a valid 64-character lowercase hex SHA-256 digest.
func ValidateHash(h string) error {
	if len(h) != 64 {
		return ErrInvalidHash
	}
	for i := 0; i < len(h); i++ {
		c := h[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return ErrInvalidHash
		}
	}
	return nil
}

func (e *Engine) blobPath(hash string) string {
	shard := hash[:2]
	return filepath.Join(e.blobsDir, shard, hash+".blob")
}

// Put streams data from r to a temporary file, computes its SHA-256 digest,
// deduplicates if the content already exists, or atomically moves it into place.
func (e *Engine) Put(r io.Reader) (PutResult, error) {
	tmpFile, err := os.CreateTemp(e.tmpDir, "upload-*.part")
	if err != nil {
		return PutResult{}, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	hasher := sha256.New()
	limited := io.LimitReader(r, e.maxObjectBytes+1)
	written, err := io.Copy(io.MultiWriter(tmpFile, hasher), limited)
	if err != nil {
		return PutResult{}, fmt.Errorf("write temp blob: %w", err)
	}
	if written > e.maxObjectBytes {
		return PutResult{}, ErrPayloadTooBig
	}
	if err := tmpFile.Sync(); err != nil {
		return PutResult{}, fmt.Errorf("fsync temp blob: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return PutResult{}, fmt.Errorf("close temp blob: %w", err)
	}

	digest := hex.EncodeToString(hasher.Sum(nil))
	finalPath := e.blobPath(digest)

	e.mu.Lock()
	defer e.mu.Unlock()

	e.totalPuts++
	if info, statErr := os.Stat(finalPath); statErr == nil && !info.IsDir() {
		e.dedupHits++
		e.dedupBytesSaved += written
		return PutResult{
			SHA256:       digest,
			Size:         written,
			Deduplicated: true,
		}, nil
	}

	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		return PutResult{}, fmt.Errorf("create shard dir: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return PutResult{}, fmt.Errorf("commit blob rename: %w", err)
	}

	return PutResult{
		SHA256:       digest,
		Size:         written,
		Deduplicated: false,
	}, nil
}

// PutBytes is a convenience wrapper around Put for in-memory byte slices.
func (e *Engine) PutBytes(data []byte) (PutResult, error) {
	return e.Put(bytes.NewReader(data))
}

// Exists checks if a blob with the given SHA-256 digest is present on disk.
func (e *Engine) Exists(hash string) bool {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if ValidateHash(hash) != nil {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	info, err := os.Stat(e.blobPath(hash))
	return err == nil && !info.IsDir()
}

// Get reads the complete contents of a blob by its SHA-256 digest.
func (e *Engine) Get(hash string) ([]byte, error) {
	rc, _, err := e.Open(hash)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// Open opens a read stream to the blob and returns its byte size.
func (e *Engine) Open(hash string) (io.ReadCloser, int64, error) {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if err := ValidateHash(hash); err != nil {
		return nil, 0, err
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	path := e.blobPath(hash)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, ErrBlobNotFound
		}
		return nil, 0, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, err
	}
	return f, info.Size(), nil
}

// DeleteBlob removes an unreferenced blob from disk.
func (e *Engine) DeleteBlob(hash string) error {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if err := ValidateHash(hash); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	err := os.Remove(e.blobPath(hash))
	if errors.Is(err, os.ErrNotExist) {
		return ErrBlobNotFound
	}
	return err
}

// CorruptBlobForTest overwrites a stored blob's content on disk to simulate silent bit-rot.
func (e *Engine) CorruptBlobForTest(hash string, raw []byte) error {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if err := ValidateHash(hash); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	path := e.blobPath(hash)
	if _, err := os.Stat(path); err != nil {
		return ErrBlobNotFound
	}
	return os.WriteFile(path, raw, 0o644)
}

// ScrubAll reads every stored CAS blob on disk, recomputes its SHA-256 digest,
// and detects any silent bit-rot or file corruption.
func (e *Engine) ScrubAll() (ScrubReport, error) {
	start := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()

	report := ScrubReport{
		CorruptedBlobs: []CorruptedBlob{},
		ScrubbedAt:     start.UTC(),
		Healthy:        true,
	}

	var blobFiles []string
	err := filepath.WalkDir(e.blobsDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".blob") {
			blobFiles = append(blobFiles, path)
		}
		return nil
	})
	if err != nil {
		return report, fmt.Errorf("walk blobs dir: %w", err)
	}

	sort.Strings(blobFiles)
	for _, path := range blobFiles {
		base := filepath.Base(path)
		expectedHash := strings.TrimSuffix(base, ".blob")
		report.TotalBlobs++

		f, openErr := os.Open(path)
		if openErr != nil {
			report.CorruptedBlobs = append(report.CorruptedBlobs, CorruptedBlob{
				ExpectedSHA256: expectedHash,
				Error:          openErr.Error(),
			})
			continue
		}

		h := sha256.New()
		n, copyErr := io.Copy(h, f)
		_ = f.Close()
		report.TotalBytesVerified += n

		if copyErr != nil {
			report.CorruptedBlobs = append(report.CorruptedBlobs, CorruptedBlob{
				ExpectedSHA256: expectedHash,
				Size:           n,
				Error:          copyErr.Error(),
			})
			continue
		}

		actualHash := hex.EncodeToString(h.Sum(nil))
		if actualHash != expectedHash {
			report.CorruptedBlobs = append(report.CorruptedBlobs, CorruptedBlob{
				ExpectedSHA256: expectedHash,
				ActualSHA256:   actualHash,
				Size:           n,
				Error:          "sha256 digest mismatch (bit-rot detected)",
			})
		} else {
			report.VerifiedBlobs++
		}
	}

	report.CorruptedCount = len(report.CorruptedBlobs)
	report.Healthy = report.CorruptedCount == 0
	report.DurationMs = time.Since(start).Milliseconds()
	reportCopy := report
	e.lastScrub = &reportCopy
	return report, nil
}

// Stats returns physical storage metrics and deduplication counters.
func (e *Engine) Stats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var uniqueBlobs int
	var physicalBytes int64
	_ = filepath.WalkDir(e.blobsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".blob") {
			return nil
		}
		uniqueBlobs++
		if info, statErr := d.Info(); statErr == nil {
			physicalBytes += info.Size()
		}
		return nil
	})

	var lastScrub *ScrubReport
	if e.lastScrub != nil {
		cp := *e.lastScrub
		lastScrub = &cp
	}

	return EngineStats{
		UniqueBlobs:     uniqueBlobs,
		PhysicalBytes:   physicalBytes,
		TotalPuts:       e.totalPuts,
		DedupHits:       e.dedupHits,
		DedupBytesSaved: e.dedupBytesSaved,
		LastScrub:       lastScrub,
	}
}

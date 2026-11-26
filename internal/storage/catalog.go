package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrBucketNotFound      = errors.New("bucket not found")
	ErrBucketAlreadyExists = errors.New("bucket already exists")
	ErrInvalidBucketName   = errors.New("invalid bucket name: must be 3-63 lowercase alphanumeric, hyphen, or dot characters")
	ErrQuotaExceeded       = errors.New("bucket storage quota exceeded")
	ErrObjectNotFound      = errors.New("object not found")
	ErrInvalidObjectKey    = errors.New("invalid object key")
)

var bucketNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// Bucket represents an S3-compatible bucket with governance and quota settings.
type Bucket struct {
	Name              string    `json:"name"`
	QuotaMB           int64     `json:"quota_mb"`
	VersioningEnabled bool      `json:"versioning_enabled"`
	PublicRead        bool      `json:"public_read"`
	RetentionDays     int       `json:"retention_days"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	ObjectCount       int       `json:"object_count"`
	VersionCount      int       `json:"version_count"`
	LogicalBytes      int64     `json:"logical_bytes"`
}

// ObjectVersion represents a single immutable version (or delete marker) of an object key.
type ObjectVersion struct {
	VersionID      string            `json:"version_id"`
	Bucket         string            `json:"bucket"`
	Key            string            `json:"key"`
	SHA256         string            `json:"sha256"`
	Size           int64             `json:"size"`
	ContentType    string            `json:"content_type"`
	Tags           map[string]string `json:"tags,omitempty"`
	IsLatest       bool              `json:"is_latest"`
	IsDeleteMarker bool              `json:"is_delete_marker"`
	Deduplicated   bool              `json:"deduplicated"`
	CreatedAt      time.Time         `json:"created_at"`
}

// AuditEvent logs every governance, bucket, object, presign, or scrub operation.
type AuditEvent struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Bucket    string    `json:"bucket,omitempty"`
	Key       string    `json:"key,omitempty"`
	VersionID string    `json:"version_id,omitempty"`
	Actor     string    `json:"actor"`
	Status    string    `json:"status"`
	Details   string    `json:"details,omitempty"`
}

// CatalogSnapshot represents the persistent state of the catalog for WAL storage and backups.
type CatalogSnapshot struct {
	SchemaVersion int                          `json:"schema_version"`
	ExportedAt    time.Time                    `json:"exported_at"`
	Buckets       map[string]*Bucket           `json:"buckets"`
	Objects       map[string][]*ObjectVersion  `json:"objects"` // key: "bucket/key" -> versions newest first
	Audits        []AuditEvent                 `json:"audits"`
}

// CatalogSummary aggregates logical metadata statistics.
type CatalogSummary struct {
	TotalBuckets      int   `json:"total_buckets"`
	TotalActiveKeys   int   `json:"total_active_keys"`
	TotalVersions     int   `json:"total_versions"`
	TotalLogicalBytes int64 `json:"total_logical_bytes"`
	AuditLogCount     int   `json:"audit_log_count"`
}

// Catalog manages buckets, object version histories, quotas, and audit logs with atomic persistence.
type Catalog struct {
	mu       sync.RWMutex
	filePath string
	state    CatalogSnapshot
}

// NewCatalog loads or initializes a transactional metadata catalog at dataDir/catalog.json.
func NewCatalog(dataDir string) (*Catalog, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create catalog dir: %w", err)
	}
	filePath := filepath.Join(dataDir, "catalog.json")
	c := &Catalog{
		filePath: filePath,
		state: CatalogSnapshot{
			SchemaVersion: 1,
			Buckets:       make(map[string]*Bucket),
			Objects:       make(map[string][]*ObjectVersion),
			Audits:        []AuditEvent{},
		},
	}

	if raw, err := os.ReadFile(filePath); err == nil && len(raw) > 0 {
		var loaded CatalogSnapshot
		if err := json.Unmarshal(raw, &loaded); err != nil {
			return nil, fmt.Errorf("corrupt catalog.json: %w", err)
		}
		if loaded.Buckets == nil {
			loaded.Buckets = make(map[string]*Bucket)
		}
		if loaded.Objects == nil {
			loaded.Objects = make(map[string][]*ObjectVersion)
		}
		if loaded.SchemaVersion == 0 {
			loaded.SchemaVersion = 1
		}
		c.state = loaded
		c.recomputeAllBucketStatsLocked()
	} else {
		if err := c.persistLocked(); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%d_%s", prefix, time.Now().UTC().UnixNano(), hex.EncodeToString(b))
}

func compositeKey(bucket, key string) string {
	return bucket + "\x00" + key
}

func splitCompositeKey(ck string) (string, string) {
	parts := strings.SplitN(ck, "\x00", 2)
	if len(parts) != 2 {
		// Backward compatibility if slash was used in snapshot
		slashParts := strings.SplitN(ck, "/", 2)
		if len(slashParts) == 2 {
			return slashParts[0], slashParts[1]
		}
		return ck, ""
	}
	return parts[0], parts[1]
}

func (c *Catalog) persistLocked() error {
	c.state.ExportedAt = time.Now().UTC()
	data, err := json.MarshalIndent(c.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal catalog: %w", err)
	}
	tmpPath := c.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write temp catalog: %w", err)
	}
	if err := os.Rename(tmpPath, c.filePath); err != nil {
		return fmt.Errorf("commit catalog: %w", err)
	}
	return nil
}

func (c *Catalog) recomputeBucketStatsLocked(bucketName string) {
	b, ok := c.state.Buckets[bucketName]
	if !ok {
		return
	}
	var objCount, verCount int
	var logicalBytes int64
	for ck, versions := range c.state.Objects {
		bName, _ := splitCompositeKey(ck)
		if bName != bucketName || len(versions) == 0 {
			continue
		}
		for i, v := range versions {
			v.IsLatest = (i == 0)
			verCount++
			if !v.IsDeleteMarker {
				logicalBytes += v.Size
			}
		}
		if !versions[0].IsDeleteMarker {
			objCount++
		}
	}
	b.ObjectCount = objCount
	b.VersionCount = verCount
	b.LogicalBytes = logicalBytes
}

func (c *Catalog) recomputeAllBucketStatsLocked() {
	for name := range c.state.Buckets {
		c.recomputeBucketStatsLocked(name)
	}
}

// RecordAudit appends an audit entry and persists the catalog.
func (c *Catalog) RecordAudit(action, bucket, key, versionID, actor, status, details string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.appendAuditLocked(action, bucket, key, versionID, actor, status, details)
	_ = c.persistLocked()
}

func (c *Catalog) appendAuditLocked(action, bucket, key, versionID, actor, status, details string) {
	if actor == "" {
		actor = "system"
	}
	ev := AuditEvent{
		ID:        newID("aud"),
		Timestamp: time.Now().UTC(),
		Action:    action,
		Bucket:    bucket,
		Key:       key,
		VersionID: versionID,
		Actor:     actor,
		Status:    status,
		Details:   details,
	}
	c.state.Audits = append([]AuditEvent{ev}, c.state.Audits...)
	if len(c.state.Audits) > 250 {
		c.state.Audits = c.state.Audits[:250]
	}
}

// CreateBucket creates a new bucket with quota, versioning, public read, and retention policies.
func (c *Catalog) CreateBucket(name string, quotaMB int64, versioning, publicRead bool, retentionDays int, actor string) (Bucket, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !bucketNameRegex.MatchString(name) {
		return Bucket{}, ErrInvalidBucketName
	}
	if quotaMB <= 0 {
		quotaMB = 100 // default 100 MB
	}
	if retentionDays < 0 {
		retentionDays = 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.state.Buckets[name]; exists {
		return Bucket{}, ErrBucketAlreadyExists
	}

	now := time.Now().UTC()
	b := &Bucket{
		Name:              name,
		QuotaMB:           quotaMB,
		VersioningEnabled: versioning,
		PublicRead:        publicRead,
		RetentionDays:     retentionDays,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	c.state.Buckets[name] = b
	c.appendAuditLocked("CREATE_BUCKET", name, "", "", actor, "SUCCESS",
		fmt.Sprintf("quota=%dMB versioning=%v public_read=%v retention=%dd", quotaMB, versioning, publicRead, retentionDays))
	if err := c.persistLocked(); err != nil {
		return Bucket{}, err
	}
	return *b, nil
}

// UpdateBucket updates governance policy fields on an existing bucket.
func (c *Catalog) UpdateBucket(name string, quotaMB int64, versioning, publicRead bool, retentionDays int, actor string) (Bucket, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	c.mu.Lock()
	defer c.mu.Unlock()

	b, exists := c.state.Buckets[name]
	if !exists {
		return Bucket{}, ErrBucketNotFound
	}
	if quotaMB > 0 {
		b.QuotaMB = quotaMB
	}
	b.VersioningEnabled = versioning
	b.PublicRead = publicRead
	if retentionDays >= 0 {
		b.RetentionDays = retentionDays
	}
	b.UpdatedAt = time.Now().UTC()
	c.recomputeBucketStatsLocked(name)
	c.appendAuditLocked("UPDATE_BUCKET", name, "", "", actor, "SUCCESS",
		fmt.Sprintf("quota=%dMB versioning=%v public_read=%v retention=%dd", b.QuotaMB, b.VersioningEnabled, b.PublicRead, b.RetentionDays))
	if err := c.persistLocked(); err != nil {
		return Bucket{}, err
	}
	return *b, nil
}

// GetBucket retrieves a single bucket by name.
func (c *Catalog) GetBucket(name string) (Bucket, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	c.mu.RLock()
	defer c.mu.RUnlock()

	b, exists := c.state.Buckets[name]
	if !exists {
		return Bucket{}, ErrBucketNotFound
	}
	return *b, nil
}

// ListBuckets returns all buckets sorted alphabetically.
func (c *Catalog) ListBuckets() []Bucket {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]Bucket, 0, len(c.state.Buckets))
	for _, b := range c.state.Buckets {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// DeleteBucket deletes a bucket and all of its object metadata records.
func (c *Catalog) DeleteBucket(name string, actor string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.state.Buckets[name]; !exists {
		return ErrBucketNotFound
	}
	for ck := range c.state.Objects {
		bName, _ := splitCompositeKey(ck)
		if bName == name {
			delete(c.state.Objects, ck)
		}
	}
	delete(c.state.Buckets, name)
	c.appendAuditLocked("DELETE_BUCKET", name, "", "", actor, "SUCCESS", "bucket and metadata deleted")
	return c.persistLocked()
}

// CheckQuota verifies whether adding incomingBytes to bucket would exceed its QuotaMB.
func (c *Catalog) CheckQuota(bucketName string, incomingBytes int64) error {
	bucketName = strings.ToLower(strings.TrimSpace(bucketName))
	c.mu.RLock()
	defer c.mu.RUnlock()

	b, exists := c.state.Buckets[bucketName]
	if !exists {
		return ErrBucketNotFound
	}
	maxBytes := b.QuotaMB * 1024 * 1024
	if b.LogicalBytes+incomingBytes > maxBytes {
		return fmt.Errorf("%w: bucket %q uses %d bytes + incoming %d bytes > quota %d bytes (%d MB)",
			ErrQuotaExceeded, bucketName, b.LogicalBytes, incomingBytes, maxBytes, b.QuotaMB)
	}
	return nil
}

// PutObjectVersion stores metadata for an uploaded object, creating a new version if versioning is enabled
// or replacing the current version if versioning is disabled.
func (c *Catalog) PutObjectVersion(bucketName, key, sha256Hex string, size int64, contentType string, tags map[string]string, deduplicated bool, actor string) (ObjectVersion, error) {
	bucketName = strings.ToLower(strings.TrimSpace(bucketName))
	key = strings.TrimPrefix(strings.TrimSpace(key), "/")
	if key == "" || strings.Contains(key, "\x00") || strings.Contains(key, "..") {
		return ObjectVersion{}, ErrInvalidObjectKey
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	b, exists := c.state.Buckets[bucketName]
	if !exists {
		return ObjectVersion{}, ErrBucketNotFound
	}

	ck := compositeKey(bucketName, key)
	existing := c.state.Objects[ck]

	// Compute projected logical bytes after write
	projectedBytes := b.LogicalBytes + size
	if !b.VersioningEnabled && len(existing) > 0 {
		for _, oldV := range existing {
			if !oldV.IsDeleteMarker {
				projectedBytes -= oldV.Size
			}
		}
	}
	maxBytes := b.QuotaMB * 1024 * 1024
	if projectedBytes > maxBytes {
		c.appendAuditLocked("PUT_OBJECT", bucketName, key, "", actor, "QUOTA_EXCEEDED",
			fmt.Sprintf("size=%d projected=%d quota=%d", size, projectedBytes, maxBytes))
		_ = c.persistLocked()
		return ObjectVersion{}, fmt.Errorf("%w: projected %d bytes exceeds quota %d MB", ErrQuotaExceeded, projectedBytes, b.QuotaMB)
	}

	cleanTags := make(map[string]string, len(tags))
	for k, v := range tags {
		k = strings.TrimSpace(k)
		if k != "" {
			cleanTags[k] = strings.TrimSpace(v)
		}
	}

	verID := "null"
	if b.VersioningEnabled {
		verID = newID("v")
	} else {
		verID = newID("v1")
	}

	newVer := &ObjectVersion{
		VersionID:      verID,
		Bucket:         bucketName,
		Key:            key,
		SHA256:         sha256Hex,
		Size:           size,
		ContentType:    contentType,
		Tags:           cleanTags,
		IsLatest:       true,
		IsDeleteMarker: false,
		Deduplicated:   deduplicated,
		CreatedAt:      time.Now().UTC(),
	}

	if b.VersioningEnabled {
		for _, old := range existing {
			old.IsLatest = false
		}
		c.state.Objects[ck] = append([]*ObjectVersion{newVer}, existing...)
	} else {
		c.state.Objects[ck] = []*ObjectVersion{newVer}
	}

	b.UpdatedAt = time.Now().UTC()
	c.recomputeBucketStatsLocked(bucketName)
	c.appendAuditLocked("PUT_OBJECT", bucketName, key, newVer.VersionID, actor, "SUCCESS",
		fmt.Sprintf("sha256=%s size=%d dedup=%v", sha256Hex[:12], size, deduplicated))
	if err := c.persistLocked(); err != nil {
		return ObjectVersion{}, err
	}
	return *newVer, nil
}

// GetObjectVersion retrieves either the latest object version (when versionID == "")
// or a specific historical version.
func (c *Catalog) GetObjectVersion(bucketName, key, versionID string) (ObjectVersion, error) {
	bucketName = strings.ToLower(strings.TrimSpace(bucketName))
	key = strings.TrimPrefix(strings.TrimSpace(key), "/")

	c.mu.RLock()
	defer c.mu.RUnlock()

	if _, exists := c.state.Buckets[bucketName]; !exists {
		return ObjectVersion{}, ErrBucketNotFound
	}
	versions := c.state.Objects[compositeKey(bucketName, key)]
	if len(versions) == 0 {
		return ObjectVersion{}, ErrObjectNotFound
	}

	if versionID == "" {
		latest := versions[0]
		if latest.IsDeleteMarker {
			return ObjectVersion{}, ErrObjectNotFound
		}
		return *latest, nil
	}

	for _, v := range versions {
		if v.VersionID == versionID {
			if v.IsDeleteMarker {
				return ObjectVersion{}, ErrObjectNotFound
			}
			return *v, nil
		}
	}
	return ObjectVersion{}, ErrObjectNotFound
}

// ListObjects lists object versions in a bucket, optionally filtered by key prefix and including all historical versions.
func (c *Catalog) ListObjects(bucketName, prefix string, includeVersions bool) ([]ObjectVersion, error) {
	bucketName = strings.ToLower(strings.TrimSpace(bucketName))
	prefix = strings.TrimPrefix(strings.TrimSpace(prefix), "/")

	c.mu.RLock()
	defer c.mu.RUnlock()

	if _, exists := c.state.Buckets[bucketName]; !exists {
		return nil, ErrBucketNotFound
	}

	var results []ObjectVersion
	for ck, versions := range c.state.Objects {
		bName, key := splitCompositeKey(ck)
		if bName != bucketName || len(versions) == 0 {
			continue
		}
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		if includeVersions {
			for _, v := range versions {
				results = append(results, *v)
			}
		} else {
			latest := versions[0]
			if !latest.IsDeleteMarker {
				results = append(results, *latest)
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Key == results[j].Key {
			return results[i].CreatedAt.After(results[j].CreatedAt)
		}
		return results[i].Key < results[j].Key
	})
	return results, nil
}

// DeleteObject deletes an object or specific version. When bucket versioning is enabled and versionID is empty,
// a delete marker is inserted at the top of the version stack so historical versions remain recoverable.
func (c *Catalog) DeleteObject(bucketName, key, versionID, actor string) (*ObjectVersion, error) {
	bucketName = strings.ToLower(strings.TrimSpace(bucketName))
	key = strings.TrimPrefix(strings.TrimSpace(key), "/")

	c.mu.Lock()
	defer c.mu.Unlock()

	b, exists := c.state.Buckets[bucketName]
	if !exists {
		return nil, ErrBucketNotFound
	}

	ck := compositeKey(bucketName, key)
	versions := c.state.Objects[ck]
	if len(versions) == 0 {
		return nil, ErrObjectNotFound
	}

	if versionID != "" {
		idx := -1
		var removed *ObjectVersion
		for i, v := range versions {
			if v.VersionID == versionID {
				idx = i
				cp := *v
				removed = &cp
				break
			}
		}
		if idx == -1 {
			return nil, ErrObjectNotFound
		}
		versions = append(versions[:idx], versions[idx+1:]...)
		if len(versions) == 0 {
			delete(c.state.Objects, ck)
		} else {
			for i, v := range versions {
				v.IsLatest = (i == 0)
			}
			c.state.Objects[ck] = versions
		}
		c.recomputeBucketStatsLocked(bucketName)
		c.appendAuditLocked("DELETE_VERSION", bucketName, key, versionID, actor, "SUCCESS", "permanent version delete")
		if err := c.persistLocked(); err != nil {
			return nil, err
		}
		return removed, nil
	}

	if b.VersioningEnabled {
		for _, v := range versions {
			v.IsLatest = false
		}
		marker := &ObjectVersion{
			VersionID:      newID("dm"),
			Bucket:         bucketName,
			Key:            key,
			ContentType:    "application/x-delete-marker",
			IsLatest:       true,
			IsDeleteMarker: true,
			CreatedAt:      time.Now().UTC(),
		}
		c.state.Objects[ck] = append([]*ObjectVersion{marker}, versions...)
		c.recomputeBucketStatsLocked(bucketName)
		c.appendAuditLocked("DELETE_OBJECT", bucketName, key, marker.VersionID, actor, "SUCCESS", "created delete marker")
		if err := c.persistLocked(); err != nil {
			return nil, err
		}
		cp := *marker
		return &cp, nil
	}

	removed := *versions[0]
	delete(c.state.Objects, ck)
	c.recomputeBucketStatsLocked(bucketName)
	c.appendAuditLocked("DELETE_OBJECT", bucketName, key, removed.VersionID, actor, "SUCCESS", "permanent object delete")
	if err := c.persistLocked(); err != nil {
		return nil, err
	}
	return &removed, nil
}

// ListAudits returns the most recent audit log events up to limit.
func (c *Catalog) ListAudits(limit int) []AuditEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 || limit > len(c.state.Audits) {
		limit = len(c.state.Audits)
	}
	out := make([]AuditEvent, limit)
	copy(out, c.state.Audits[:limit])
	return out
}

// Summary computes aggregated logical storage statistics across all buckets.
func (c *Catalog) Summary() CatalogSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var sum CatalogSummary
	sum.TotalBuckets = len(c.state.Buckets)
	sum.AuditLogCount = len(c.state.Audits)
	for _, b := range c.state.Buckets {
		sum.TotalActiveKeys += b.ObjectCount
		sum.TotalVersions += b.VersionCount
		sum.TotalLogicalBytes += b.LogicalBytes
	}
	return sum
}

// ExportBackup returns a deep-copied snapshot of the entire catalog.
func (c *Catalog) ExportBackup(actor string) (CatalogSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.appendAuditLocked("BACKUP_CATALOG", "", "", "", actor, "SUCCESS", "exported metadata snapshot")
	_ = c.persistLocked()

	raw, err := json.Marshal(c.state)
	if err != nil {
		return CatalogSnapshot{}, err
	}
	var snap CatalogSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return CatalogSnapshot{}, err
	}
	return snap, nil
}

// ImportRestore replaces the catalog state with a validated snapshot and persists it atomically.
func (c *Catalog) ImportRestore(snap CatalogSnapshot, actor string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if snap.Buckets == nil {
		snap.Buckets = make(map[string]*Bucket)
	}
	if snap.Objects == nil {
		snap.Objects = make(map[string][]*ObjectVersion)
	}
	if snap.SchemaVersion == 0 {
		snap.SchemaVersion = 1
	}
	c.state = snap
	c.recomputeAllBucketStatsLocked()
	c.appendAuditLocked("RESTORE_CATALOG", "", "", "", actor, "SUCCESS",
		fmt.Sprintf("restored %d buckets", len(c.state.Buckets)))
	return c.persistLocked()
}

# Changelog

All notable changes to **NimbusStore** are documented in this file.

## [v0.1.0] - 2026-11-26

### Added
- **Content-Addressable Storage (CAS) Engine (`internal/cas`)**:
  - Streaming SHA-256 blob ingestion with two-level hex directory sharding (`blobs/<prefix>/<sha256>.blob`) and atomic temporary-file-to-final rename writes.
  - Automatic cross-object, cross-version, and cross-bucket deduplication with real-time `dedup_hits` and `dedup_bytes_saved` telemetry.
  - Full-disk SHA-256 bit-rot integrity scrubber (`ScrubAll()`) to detect silent disk corruption.
- **HMAC-SHA256 Presigned URL Authority (`internal/signing`)**:
  - S3-style presigned URL generation and constant-time verification bound to HTTP method (`GET`/`PUT`), bucket, object key, optional `versionId`, and expiration timestamp.
- **Transactional Bucket & Version Catalog (`internal/storage`)**:
  - Bucket governance policies with `quota_mb`, `versioning_enabled`, `public_read`, and `retention_days`.
  - Immutable object version history (`VersionID`, `IsLatest`, `IsDeleteMarker`), custom metadata tags, and structured access audit trail.
  - Atomic JSON snapshot persistence plus online backup (`GET /api/v1/admin/backup`) and restore (`POST /api/v1/admin/restore`).
- **REST Control Plane, S3 Data Plane & Governance Web Console (`internal/api`, `internal/web`)**:
  - S3-compatible `/s3/{bucket}/{key}` endpoints (`GET`, `HEAD`, `PUT`, `DELETE`) and `/api/v1/*` governance APIs with Bearer, Access-Key/Secret-Key, and Presigned URL authentication plus token-bucket rate limiting.
  - Custom-branded **NimbusStore Object & Bucket Governance Console** with dark/light theme toggle, deduplication telemetry, bucket quota bars, version history explorer, interactive object uploader, presigned URL studio, bit-rot scrubber, and audit log viewer.

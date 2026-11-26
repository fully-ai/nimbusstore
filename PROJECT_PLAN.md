# NimbusStore — Engineering & Product Plan

## 1. Product Vision
**NimbusStore** is a self-hosted, S3-compatible Content-Addressable Object Storage (CAS) engine, immutable versioning system, HMAC-SHA256 presigned URL broker, bit-rot integrity scrubber, and bucket governance platform written in zero-dependency standard-library Go.

## 2. Core Architectural Pillars
1. **SHA-256 Content-Addressable Storage (`internal/cas`)**:
   - Every uploaded payload is hashed via streaming SHA-256 into a staging temp file and atomically renamed into a two-level hex shard directory (`blobs/<prefix>/<sha256>.blob`).
   - Automatic cross-key, cross-version, and cross-bucket deduplication eliminates redundant physical disk writes while tracking deduplication hits and bytes saved.
   - Active bit-rot scrubbing (`ScrubAll()`) re-verifies every stored `.blob` file against its content-addressed SHA-256 filename digest.
2. **S3-Style Presigned URL Authority (`internal/signing`)**:
   - Stateless HMAC-SHA256 canonical signing bound to HTTP method (`GET` or `PUT`), bucket, object key, optional `versionId`, and Unix expiration timestamp.
   - Constant-time MAC verification (`hmac.Equal`) prevents timing attacks and parameter tampering.
3. **Transactional Metadata & Governance Catalog (`internal/storage`)**:
   - Manages bucket policies (`quota_mb`, `versioning_enabled`, `public_read`, `retention_days`), immutable object version stacks, delete markers, custom tags, and governance audit logs.
   - Atomic JSON write-ahead snapshot persistence plus full online JSON backup and restore.
4. **REST Control Plane, S3 Data Plane & Governance Web Console (`internal/api`, `internal/web`)**:
   - Dual API surface supporting both `/api/v1/*` JSON governance endpoints and `/s3/{bucket}/{key}` S3-compatible streaming transfers.
   - Built-in Sky-Blue/Cobalt/Slate **NimbusStore Object & Bucket Governance Console** with live deduplication telemetry, bucket quota bars, version history explorer, interactive uploader, presigned URL studio, bit-rot scrubber, and audit trail.

## 3. Milestones & Verification Matrix
- [x] M1: Content-Addressable Storage engine with atomic writes, deduplication counters, and bit-rot scrubber (`internal/cas`).
- [x] M2: HMAC-SHA256 Presigned URL generator and constant-time verifier (`internal/signing`).
- [x] M3: Persistent Bucket, Quota, Version Stack & Audit Catalog (`internal/storage`).
- [x] M4: REST + S3 HTTP Server with Bearer & Access-Key/Secret-Key auth and rate limiting (`internal/api`).
- [x] M5: Custom-branded NimbusStore Object & Bucket Governance Console (`internal/web`).
- [x] M6: Unit & integration test suite (`go vet ./...`, `go test -v -race ./...`), Dockerfile, Docker Compose, and GitHub Actions CI.

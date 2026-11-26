package cas

import (
	"bytes"
	"testing"
)

func TestEnginePutDeduplicationAndGet(t *testing.T) {
	dir := t.TempDir()
	eng, err := NewEngine(dir, 1024*1024)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	payload := []byte("deterministic-content-addressed-payload-v1")
	res1, err := eng.PutBytes(payload)
	if err != nil {
		t.Fatalf("first PutBytes failed: %v", err)
	}
	if res1.Deduplicated {
		t.Fatalf("expected first put not to be deduplicated")
	}
	if res1.Size != int64(len(payload)) {
		t.Fatalf("expected size %d, got %d", len(payload), res1.Size)
	}

	// Second put of identical content must deduplicate
	res2, err := eng.PutBytes(payload)
	if err != nil {
		t.Fatalf("second PutBytes failed: %v", err)
	}
	if !res2.Deduplicated {
		t.Fatalf("expected second identical put to be deduplicated")
	}
	if res1.SHA256 != res2.SHA256 {
		t.Fatalf("digest mismatch: %s vs %s", res1.SHA256, res2.SHA256)
	}

	got, err := eng.Get(res1.SHA256)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("retrieved payload mismatch")
	}

	stats := eng.Stats()
	if stats.UniqueBlobs != 1 {
		t.Fatalf("expected 1 unique blob on disk, got %d", stats.UniqueBlobs)
	}
	if stats.DedupHits != 1 {
		t.Fatalf("expected 1 dedup hit, got %d", stats.DedupHits)
	}
	if stats.DedupBytesSaved != int64(len(payload)) {
		t.Fatalf("expected %d dedup bytes saved, got %d", len(payload), stats.DedupBytesSaved)
	}
}

func TestEngineBitRotScrubDetection(t *testing.T) {
	dir := t.TempDir()
	eng, err := NewEngine(dir, 1024*1024)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	resA, err := eng.PutBytes([]byte("healthy-blob-alpha"))
	if err != nil {
		t.Fatalf("put alpha: %v", err)
	}
	resB, err := eng.PutBytes([]byte("healthy-blob-bravo"))
	if err != nil {
		t.Fatalf("put bravo: %v", err)
	}

	rep1, err := eng.ScrubAll()
	if err != nil {
		t.Fatalf("ScrubAll failed: %v", err)
	}
	if !rep1.Healthy || rep1.VerifiedBlobs != 2 || rep1.CorruptedCount != 0 {
		t.Fatalf("expected healthy scrub report with 2 verified blobs, got %+v", rep1)
	}

	// Simulate silent disk corruption (bit-rot) on blob B
	if err := eng.CorruptBlobForTest(resB.SHA256, []byte("corrupted-bit-rot-bytes")); err != nil {
		t.Fatalf("CorruptBlobForTest failed: %v", err)
	}

	rep2, err := eng.ScrubAll()
	if err != nil {
		t.Fatalf("ScrubAll after corruption failed: %v", err)
	}
	if rep2.Healthy {
		t.Fatalf("expected scrub report to flag corruption")
	}
	if rep2.VerifiedBlobs != 1 || rep2.CorruptedCount != 1 {
		t.Fatalf("expected 1 verified and 1 corrupted blob, got %+v", rep2)
	}
	if rep2.CorruptedBlobs[0].ExpectedSHA256 != resB.SHA256 {
		t.Fatalf("expected corrupted hash %s, got %s", resB.SHA256, rep2.CorruptedBlobs[0].ExpectedSHA256)
	}
	if !eng.Exists(resA.SHA256) {
		t.Fatalf("expected blob A to exist")
	}
}

func TestEnginePayloadLimitAndInvalidHash(t *testing.T) {
	dir := t.TempDir()
	eng, err := NewEngine(dir, 16)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	if _, err := eng.PutBytes([]byte("this-payload-exceeds-16-bytes")); err != ErrPayloadTooBig {
		t.Fatalf("expected ErrPayloadTooBig, got %v", err)
	}

	if _, err := eng.Get("../../etc/passwd"); err != ErrInvalidHash {
		t.Fatalf("expected ErrInvalidHash for path traversal attempt, got %v", err)
	}
}

package main

import (
	"bufio"
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/image"
)

func TestReadImageLoadedRefSkipsStatusLines(t *testing.T) {
	ref := mustParseReference(t, "ghcr.io/example/app:v1.0.0")
	const digest = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	stream := `{"status":"Pull complete","id":"abc123"}` + "\n" +
		`{"status":"Extracting","id":"def456","progressDetail":{}}` + "\n" +
		`{"stream":"Loaded image ID: sha256:` + digest + `"}` + "\n"

	loaded, err := readImageLoadedRef(
		context.Background(),
		ref,
		bufio.NewReader(strings.NewReader(stream)),
	)
	if err != nil {
		t.Fatalf("readImageLoadedRef failed: %v", err)
	}
	if got := loaded.String(); got != "ghcr.io/example/app@sha256:"+digest {
		t.Fatalf("unexpected loaded ref: %q", got)
	}
}

func TestReadImageLoadedRefTaggedImage(t *testing.T) {
	ref := mustParseReference(t, "ghcr.io/example/app:v1.0.0")

	stream := `{"status":"Loading layer","id":"abc"}` + "\n" +
		`{"stream":"Loaded image: ghcr.io/example/app:latest"}` + "\n"

	loaded, err := readImageLoadedRef(
		context.Background(),
		ref,
		bufio.NewReader(strings.NewReader(stream)),
	)
	if err != nil {
		t.Fatalf("readImageLoadedRef failed: %v", err)
	}
	if got := loaded.String(); got != "ghcr.io/example/app:latest" {
		t.Fatalf("unexpected loaded ref: %q", got)
	}
}

func TestReadImageLoadedRefNoRefFallsBackToTarget(t *testing.T) {
	ref := mustParseReference(t, "ghcr.io/example/app:v1.0.0")
	// docker reported only progress lines and no loaded-ref summary
	// (image already present / loaded silently). Must fall back to the
	// requested ref instead of erroring.
	stream := `{"status":"Pull complete","id":"abc"}` + "\n"

	loaded, err := readImageLoadedRef(
		context.Background(),
		ref,
		bufio.NewReader(strings.NewReader(stream)),
	)
	if err != nil {
		t.Fatalf("readImageLoadedRef failed: %v", err)
	}
	if got := loaded.String(); got != "ghcr.io/example/app:v1.0.0" {
		t.Fatalf("unexpected loaded ref: %q", got)
	}
}

func TestReadImageLoadedRefStatusAndStreamLineIsNotProgress(t *testing.T) {
	ref := mustParseReference(t, "ghcr.io/example/app:v1.0.0")
	const digest = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	// docker 28.0.x may emit a loaded-ref summary line that also carries a
	// status field. It must still be treated as the ref, not skipped.
	stream := `{"status":"Loading layer","id":"abc"}` + "\n" +
		`{"status":"Loaded image ID: sha256:` + digest + `","stream":"Loaded image ID: sha256:` + digest + `"}` + "\n"

	loaded, err := readImageLoadedRef(
		context.Background(),
		ref,
		bufio.NewReader(strings.NewReader(stream)),
	)
	if err != nil {
		t.Fatalf("readImageLoadedRef failed: %v", err)
	}
	if got := loaded.String(); got != "ghcr.io/example/app@sha256:"+digest {
		t.Fatalf("unexpected loaded ref: %q", got)
	}
}

// TestTagImageTwoPlatforms validates the regression from the multi-platform
// build: two consecutive calls to TagImage with different refs on the same
// underlying image blob must succeed.
//
// The bug (shikanime-studio/nix-containers#68): TagImage unconditionally
// called docker.ImageRemove after tagging. In multi-platform builds, both
// platforms load the same combined ref. After the first platform's TagImage
// removes the shared image, the second platform's tag attempt fails with
// "No such image".
//
// This test requires a running docker daemon. It is skipped gracefully
// when docker is unavailable (e.g. CI runners without docker, local dev
// without daemon).
func TestTagImageTwoPlatforms(t *testing.T) {
	ctx := context.Background()
	c, err := NewContainerClient(ctx)
	if err != nil {
		t.Fatal("NewContainerClient should work without a daemon:", err)
	}

	// Pull a real image so we have a real image ID to tag.
	pullRef := mustParseReference(t, "busybox:RegressionTestTagImageTwoPlatforms")
	pullResp, err := c.docker.ImagePull(ctx, pullRef.Name(), image.PullOptions{})
	if err != nil {
		t.Skip("cannot pull busybox — test requires docker daemon:", err)
	}
	pullResp.Close()

	t.Cleanup(func() {
		c.docker.ImageRemove(
			ctx,
			"busybox:RegressionTestTagImageTwoPlatforms",
			image.RemoveOptions{Force: true},
		)
	})

	tagRefA := mustParseReference(t, "busybox:RegressionTestTagImageTwoPlatforms-platform-a")
	tagRefB := mustParseReference(t, "busybox:RegressionTestTagImageTwoPlatforms-platform-b")

	// Simulate two platform builds sharing the same image content.
	// Both refs point at the same underlying image blob (sha256).
	// First TagImage must succeed.
	if err := c.TagImage(ctx, tagRefA, tagRefA); err != nil {
		t.Fatalf("TagImage platform-a failed: %v", err)
	}

	// Second TagImage must also succeed.
	// Before the fix (removing unconditional ImageRemove), this would have
	// removed the shared image and crashed with "No such image".
	if err := c.TagImage(ctx, tagRefA, tagRefB); err != nil {
		t.Fatalf("TagImage platform-b failed: %v", err)
	}
}

package main

import (
	"bufio"
	"context"
	"strings"
	"testing"
)

func TestReadImageLoadedRefSkipsStatusLines(t *testing.T) {
	ref := mustParseReference(t, "ghcr.io/example/app:v1.0.0")
	const digest = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	// Docker load emits progress/status lines (Pull complete, Extracting, …)
	// before the tagless image ID line. The reader must skip those and only
	// return on an actual image reference.
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

func TestReadImageLoadedRefNoRefIsError(t *testing.T) {
	ref := mustParseReference(t, "ghcr.io/example/app:v1.0.0")
	stream := `{"status":"Pull complete","id":"abc"}` + "\n"

	if _, err := readImageLoadedRef(
		context.Background(),
		ref,
		bufio.NewReader(strings.NewReader(stream)),
	); err == nil {
		t.Fatal("expected error when no image reference is present in load output")
	}
}

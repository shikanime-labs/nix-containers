package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"golang.org/x/sync/errgroup"
)

var streamCommandContext = exec.CommandContext

type ContainerOption func(*containerOptions)

type containerOptions struct {
	docker    *client.Client
	keychain  authn.Keychain
	transport http.RoundTripper
	remote    []remote.Option
}

type ContainerClient struct {
	docker    *client.Client
	keychain  authn.Keychain
	transport http.RoundTripper
	remote    []remote.Option
}

type imageLoadProgress struct {
	Status         string         `json:"status"`
	Progress       string         `json:"progress"`
	ID             string         `json:"id"`
	ProgressDetail map[string]any `json:"progressDetail"`
}

type imageLoadResult struct {
	Stream string `json:"stream"`
}

func WithContainerKeychain(kc authn.Keychain) ContainerOption {
	return func(o *containerOptions) {
		o.keychain = kc
		o.remote = append(o.remote, remote.WithAuthFromKeychain(kc))
	}
}

func WithContainerDockerClient(docker *client.Client) ContainerOption {
	return func(o *containerOptions) {
		o.docker = docker
	}
}

func WithContainerTransport(t http.RoundTripper) ContainerOption {
	return func(o *containerOptions) {
		o.transport = t
		o.remote = append(o.remote, remote.WithTransport(t))
	}
}

func WithContainerRemoteOption(opt remote.Option) ContainerOption {
	return func(o *containerOptions) {
		o.remote = append(o.remote, opt)
	}
}

func makeContainerOptions(opts ...ContainerOption) *containerOptions {
	o := &containerOptions{
		keychain:  authn.DefaultKeychain,
		transport: http.DefaultTransport,
	}
	o.remote = append(o.remote, remote.WithAuthFromKeychain(o.keychain))
	o.remote = append(o.remote, remote.WithTransport(o.transport))
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func NewContainerClient(ctx context.Context, opts ...ContainerOption) (*ContainerClient, error) {
	o := makeContainerOptions(opts...)
	docker := o.docker
	if docker == nil {
		var err error
		docker, err = client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return nil, fmt.Errorf("create docker client failed: %w", err)
		}
		docker.NegotiateAPIVersion(ctx)
	}

	return &ContainerClient{
		docker:    docker,
		keychain:  o.keychain,
		transport: o.transport,
		remote:    o.remote,
	}, nil
}

func (c *ContainerClient) CheckPushPermission(ref name.Reference) error {
	if err := remote.CheckPushPermission(ref, c.keychain, c.transport); err != nil {
		return fmt.Errorf("check push permission failed: %w", err)
	}
	return nil
}

func (c *ContainerClient) TagImage(
	ctx context.Context,
	loadedRef, ref name.Reference,
) error {
	if err := c.docker.ImageTag(ctx, loadedRef.Name(), ref.Name()); err != nil {
		return fmt.Errorf("tag image failed: %w", err)
	}
	_, err := c.docker.ImageRemove(ctx, loadedRef.Name(), image.RemoveOptions{})
	if err != nil {
		return fmt.Errorf("remove image failed: %w", err)
	}
	return nil
}

func (c *ContainerClient) LoadImage(
	ctx context.Context,
	ref name.Reference,
	path string,
) (name.Reference, error) {
	slog.InfoContext(ctx, "load image", "image", ref, "path", path)

	input, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer func() { _ = input.Close() }()

	resp, err := c.docker.ImageLoad(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("docker image load failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loadedRef, err := readImageLoadedRef(ctx, ref, bufio.NewReader(resp.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to read loaded ref: %w", err)
	}

	return loadedRef, nil
}

func (c *ContainerClient) LoadStreamImage(
	ctx context.Context,
	ref name.Reference,
	path string,
) (name.Reference, error) {
	slog.InfoContext(ctx, "start stream image command", "image", ref, "path", path)
	cmd := streamCommandContext(ctx, path)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stream := bufio.NewReader(stdoutPipe)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	sc := bufio.NewScanner(stderrPipe)

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start stream command: %w", err)
	}

	wg := errgroup.Group{}
	wg.Go(func() error {
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				slog.DebugContext(ctx, line, "cmd", cmd.Path)
			}
		}
		if err = sc.Err(); err != nil {
			return fmt.Errorf("stderr scan failed: %w", err)
		}
		return nil
	})

	slog.InfoContext(ctx, "streaming image", "image", ref)
	resp, err := c.docker.ImageLoad(ctx, stream)
	if err != nil {
		return nil, fmt.Errorf("docker image load failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loadedRef, err := readImageLoadedRef(ctx, ref, bufio.NewReader(resp.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to read loaded ref: %w", err)
	}

	if err = wg.Wait(); err != nil {
		return nil, fmt.Errorf("failed to wait for stream command: %w", err)
	}
	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("failed to wait for command: %w", err)
	}

	slog.InfoContext(ctx, "stream image command completed", "image", ref, "path", path)
	return loadedRef, nil
}

func (c *ContainerClient) PushImage(ref name.Reference, path string) error {
	img, err := tarball.Image(gzipPathOpener(path), nil)
	if err != nil {
		return fmt.Errorf("load image from tarball failed: %w", err)
	}
	if err := remote.Write(ref, img, c.remote...); err != nil {
		return fmt.Errorf("push image failed: %w", err)
	}
	return nil
}

func (c *ContainerClient) PushPlatformImage(
	ref name.Reference,
	p *v1.Platform,
	path string,
) (mutate.IndexAddendum, error) {
	img, err := tarball.Image(gzipPathOpener(path), nil)
	if err != nil {
		return mutate.IndexAddendum{}, fmt.Errorf("load image from tarball failed: %w", err)
	}
	if err := remote.Write(ref, img, c.remote...); err != nil {
		return mutate.IndexAddendum{}, fmt.Errorf("push image failed: %w", err)
	}
	return mutate.IndexAddendum{
		Add:        img,
		Descriptor: v1.Descriptor{Platform: p},
	}, nil
}

func gzipPathOpener(path string) tarball.Opener {
	return func() (io.ReadCloser, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(path, ".tar.gz") {
			gr, err := gzip.NewReader(f)
			if err != nil {
				_ = f.Close()
				return nil, err
			}
			return &gzipReadCloser{Reader: gr, file: f}, nil
		}
		return f, nil
	}
}

type gzipReadCloser struct {
	*gzip.Reader
	file *os.File
}

func (g *gzipReadCloser) Close() error {
	if err := g.Reader.Close(); err != nil {
		return err
	}
	return g.file.Close()
}

func (c *ContainerClient) PushManifest(
	ref name.Reference,
	adds []mutate.IndexAddendum,
) error {
	if err := remote.WriteIndex(
		ref,
		mutate.AppendManifests(empty.Index, adds...),
		c.remote...,
	); err != nil {
		return fmt.Errorf("push manifest failed: %w", err)
	}
	return nil
}

func readImageLoadedRef(
	ctx context.Context,
	ref name.Reference,
	r *bufio.Reader,
) (name.Reference, error) {
	// Docker 28 / containerd v2 emit a tagless ref for nix's image tarball:
	// an empty "Loaded image: " line and/or a "Loaded image ID: <digest>"
	// line. Don't depend on the exact shape — extract the sha256 digest
	// wherever Docker prints it and reconstruct a taggable ref.
	var digest string
	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read line: %w", err)
		}
		if line != "" {
			var progress imageLoadProgress
			if err = json.Unmarshal([]byte(line), &progress); err == nil {
				if progress.Status == "Loading layer" {
					slog.DebugContext(
						ctx,
						"loading layer",
						"id",
						progress.ID,
						"progress",
						progress.Progress,
					)
				} else {
					var result imageLoadResult
					if err = json.Unmarshal([]byte(line), &result); err == nil {
						slog.DebugContext(ctx, "loaded image", "stream", result.Stream)
						if d, ok := extractDigest(result.Stream); ok {
							digest = d
						}
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
	}
	if digest == "" {
		return nil, fmt.Errorf("failed to read loaded ref: no digest in docker load output")
	}
	return name.ParseReference(ref.Context().String() + "@sha256:" + digest)
}

// extractDigest returns the sha256 digest from a docker load stream line,
// accepting "Loaded image ID: sha256:<digest>" as well as a bare
// "Loaded image ID: <digest>".
func extractDigest(stream string) (string, bool) {
	const prefix = "Loaded image ID: "
	s := strings.TrimSpace(stream)
	if !strings.HasPrefix(s, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(s, prefix)
	rest = strings.TrimSpace(rest)
	rest = strings.TrimPrefix(rest, "sha256:")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "", false
	}
	return rest, true
}

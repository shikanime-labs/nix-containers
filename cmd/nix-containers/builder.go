package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"golang.org/x/sync/errgroup"
)

type BuildOption func(*buildOption)

type buildOption struct {
	imageOpts []imageOption
	push      bool
}

type nixBuilderClient interface {
	BuildPlatformImage(
		context.Context,
		string,
		name.Reference,
		*v1.Platform,
		...imageOption,
	) (string, error)
}

type containerBuilderClient interface {
	CheckPushPermission(name.Reference) error
	PushImage(name.Reference, string) error
	PushPlatformImage(name.Reference, *v1.Platform, string) (mutate.IndexAddendum, error)
	PushManifest(name.Reference, []mutate.IndexAddendum) error
}

type Builder struct {
	nix       nixBuilderClient
	container containerBuilderClient
	imageOpts []imageOption
	push      bool
}

func NewBuilder(
	nix nixBuilderClient,
	container containerBuilderClient,
	opts ...BuildOption,
) *Builder {
	o := makeBuildOption(opts...)
	return &Builder{
		nix:       nix,
		container: container,
		imageOpts: o.imageOpts,
		push:      o.push,
	}
}

func WithStreamImageOption(opt imageOption) BuildOption {
	return func(o *buildOption) { o.imageOpts = append(o.imageOpts, opt) }
}

func WithPush(push bool) BuildOption {
	return func(o *buildOption) { o.push = push }
}

func makeBuildOption(opts ...BuildOption) *buildOption {
	o := &buildOption{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func (b *Builder) BuildAndPush(
	ctx context.Context,
	buildContext string,
	ref name.Reference,
	plats []*v1.Platform,
) error {
	if len(plats) == 0 {
		return fmt.Errorf("at least one platform is required")
	}
	if b.push {
		slog.InfoContext(ctx, "checking push permission", "ref", ref.Name())
		if err := b.container.CheckPushPermission(ref); err != nil {
			return err
		}
	}
	if len(plats) == 1 {
		return b.buildAndPushImage(ctx, buildContext, ref, plats[0])
	}
	return b.buildAndPushMultiplatformImage(ctx, buildContext, ref, plats)
}

// buildPlatformPath runs the nix build for one platform and returns the
// local tarball path. The tarball is pushed directly (no docker load/tag) —
// the daemon round-trip was redundant and broke on docker 28.
func (b *Builder) buildPlatformPath(
	ctx context.Context,
	buildContext string,
	ref name.Reference,
	p *v1.Platform,
) (string, error) {
	slog.InfoContext(ctx, "build image", "ref", ref.Name(), "os", p.OS, "arch", p.Architecture)
	path, err := b.nix.BuildPlatformImage(ctx, buildContext, ref, p, b.imageOpts...)
	if err != nil {
		return "", fmt.Errorf("build image failed: %w", err)
	}
	return path, nil
}

func (b *Builder) buildAndPushMultiplatformImage(
	ctx context.Context,
	buildContext string,
	ref name.Reference,
	ps []*v1.Platform,
) error {
	if !b.push {
		return fmt.Errorf(
			"multiplatform image build is only supported when pushing to remote registry",
		)
	}
	var adds []mutate.IndexAddendum
	var addsMu sync.Mutex
	slog.InfoContext(ctx, "build multiplatform image", "ref", ref.Name(), "platform_count", len(ps))
	wg, ctx := errgroup.WithContext(ctx)
	for _, p := range ps {
		p := p
		wg.Go(func() error {
			path, err := b.buildPlatformPath(ctx, buildContext, ref, p)
			if err != nil {
				return err
			}
			platformTag, err := formatPlatformReference(ref, p)
			if err != nil {
				return fmt.Errorf("format platform reference failed: %w", err)
			}
			slog.InfoContext(ctx, "push platform image", "ref", ref.Name(), "platform_ref", platformTag.Name())
			add, err := b.container.PushPlatformImage(platformTag, p, path)
			if err != nil {
				return err
			}
			addsMu.Lock()
			adds = append(adds, add)
			addsMu.Unlock()
			return nil
		})
	}
	if err := wg.Wait(); err != nil {
		return fmt.Errorf("push images failed: %w", err)
	}
	slog.InfoContext(ctx, "push manifest", "ref", ref.Name(), "platform_count", len(adds))
	if err := b.container.PushManifest(ref, adds); err != nil {
		return err
	}
	slog.InfoContext(ctx, "manifest pushed", "ref", ref.Name(), "platform_count", len(adds))
	return nil
}

func (b *Builder) buildAndPushImage(
	ctx context.Context,
	buildContext string,
	ref name.Reference,
	p *v1.Platform,
) error {
	path, err := b.buildPlatformPath(ctx, buildContext, ref, p)
	if err != nil {
		return fmt.Errorf("build flake image failed: %w", err)
	}
	if b.push {
		slog.DebugContext(ctx, "push image", "ref", ref.Name())
		if err := b.container.PushImage(ref, path); err != nil {
			return err
		}
	}
	return nil
}

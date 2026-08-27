// OpenTelemetry tracing for the nix-containers build pipeline.
//
// The OTLP/gRPC exporter reads its endpoint and TLS mode from the standard
// OTEL_EXPORTER_OTLP_* environment variables (TRACES-specific take precedence
// over the generic one). With no endpoint configured, setupTracing is a true
// no-op: it installs nothing and starts no exporter, so there is no network
// traffic and no background overhead. Export failures are logged rather than
// fatal, and exporter/provider init errors never fail a build.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// tracerName is the instrumentation scope for spans created here.
const tracerName = "github.com/shikanime-studio/nix-containers"

// version mirrors the Nix flake package version (flake.nix packages.default.version).
const version = "v0.1.0"

// setupTracing installs a process-wide TracerProvider that exports spans over
// OTLP/gRPC. It returns a shutdown function that flushes and stops the provider;
// call it on process exit.
//
// With no OTLP endpoint configured (OTEL_EXPORTER_OTLP_ENDPOINT /
// OTEL_EXPORTER_OTLP_TRACES_ENDPOINT) it is a no-op: the global no-op
// TracerProvider is left in place, so startSpan produces zero-overhead,
// network-free spans and the returned shutdown func does nothing.
func setupTracing(ctx context.Context) func(context.Context) error {
	// ponytail: single env check gates all tracing overhead; if other signal
	// types (metrics/logs) get added, extend the guard rather than always init.
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	otlpTracesEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	if otlpEndpoint == "" && otlpTracesEndpoint == "" {
		slog.DebugContext(ctx, "no OTLP endpoint configured; tracing disabled")
		return func(context.Context) error { return nil }
	}

	// Revert the TCP-only check; gRPC-level verification is needed because
	// TCP dial succeeds on any open port (e.g. SSH on :4317), but the OTLP
	// gRPC handshake then fails with connection errors.
	// ponytail: grpc health check, if health pkg is vendored
	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithTimeout(2*time.Second),
		otlptracegrpc.WithReconnectionPeriod(5*time.Second),
	)
	if err != nil {
		slog.WarnContext(ctx, "opentelemetry exporter init failed; tracing disabled", "err", err)
		return func(context.Context) error { return nil }
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName("nix-containers"),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		slog.WarnContext(ctx, "opentelemetry resource init failed; tracing disabled", "err", err)
		_ = exporter.Shutdown(ctx)
		return func(context.Context) error { return nil }
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			slog.WarnContext(ctx, "opentelemetry shutdown failed", "err", err)
			return err
		}
		return nil
	}
}

// startSpan starts a span on the package tracer, attaching the given attributes.
func startSpan(
	ctx context.Context,
	name string,
	attrs ...attribute.KeyValue,
) (context.Context, oteltrace.Span) {
	return otel.Tracer(tracerName).Start(ctx, name, oteltrace.WithAttributes(attrs...))
}

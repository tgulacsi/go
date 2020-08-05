package otel

import (
	"context"
	"net/http"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/label"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/api/propagation"
	"go.opentelemetry.io/otel/exporters/stdout"
	"go.opentelemetry.io/otel/instrumentation/httptrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/go-logfmt/logfmt"
)

type logfmtEncoder struct {
	id label.EncoderID
}

func NewLogfmtEncoder() logfmtEncoder {
	return logfmtEncoder{id: label.NewEncoderID()}
}
func (e logfmtEncoder) ID() label.EncoderID { return e.id }
func (e logfmtEncoder) Encode(it label.Iterator) string {
	var buf strings.Builder
	enc := logfmt.NewEncoder(&buf)
	for it.Next() {
		kv := it.Label()
		enc.EncodeKeyval(kv.Key, kv.Value)
	}
	enc.EndRecord()
	return buf.String()
}

type logWriter struct{ Log func(...interface{}) error }

func (lw logWriter) Write(p []byte) (int, error) {
	err := lw.Log("trace", string(p))
	return len(p), err
}

func InitTraceProvider(ctx context.Context, name string, Log func(...interface{}) error) (context.Context, error) {
	exporter, err := stdout.NewExporter(
		stdout.WithWriter(logWriter{Log: Log}),
		stdout.WithLabelEncoder(NewLogfmtEncoder()),
		stdout.WithoutTimestamps(), stdout.WithoutMetricExport(),
	)
	if err != nil {
		return ctx, err
	}
	tp, err := sdktrace.NewProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithSyncer(exporter),
	)
	if err != nil {
		return ctx, err
	}
	global.SetTraceProvider(tp)
	return context.WithValue(ctx, ckTracer, global.Tracer(name)), nil
}

var Propagators = propagation.New(
	propagation.WithExtractors(trace.DefaultHTTPPropagator()),
	propagation.WithInjectors(trace.DefaultHTTPPropagator()),
)
var httptraceOpts = []httptrace.Option{httptrace.WithPropagators(Propagators)}

func Middleware(hndl http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracer, ok := r.Context().Value(ckTracer).(trace.Tracer)
		if !ok || tracer == nil {
			fmt.Println("NO TRACER IN CONTEXT")
			hndl.ServeHTTP(w, r)
			return
		}
		tracer.WithSpan(r.Context(), r.URL.Path, func(ctx context.Context) error {
			_, _, spanContext := httptrace.Extract(ctx, r, httptraceOpts...)
			hndl.ServeHTTP(w, r.WithContext(trace.ContextWithRemoteSpanContext(ctx, spanContext)))
			return nil
		})
	})
}

type ctxKey string

const ckTracer = ctxKey("tracer")

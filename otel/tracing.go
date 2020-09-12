package otel

import (
	"context"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/label"
	"go.opentelemetry.io/otel/api/propagation"
	"go.opentelemetry.io/otel/api/trace"
	setrace "go.opentelemetry.io/otel/sdk/export/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/go-logfmt/logfmt"
)

func InitTraceProvider(ctx context.Context, name string, Log func(...interface{}) error) (context.Context, error) {
	exporter := LogExporter{Log: Log}
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
	propagation.WithExtractors(trace.DefaultHTTPPropagator(), trace.B3{}),
	propagation.WithInjectors(trace.DefaultHTTPPropagator(), trace.B3{}),
)

func ExtractHTTP(ctx context.Context, headers http.Header) context.Context {
	return propagation.ExtractHTTP(ctx, Propagators, headers)
}
func InjectHTTP(ctx context.Context, headers http.Header) {
	propagation.InjectHTTP(ctx, Propagators, headers)
}

func Middleware(hndl http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := propagation.ExtractHTTP(r.Context(), Propagators, r.Header)
		FromContext(ctx).WithSpan(ctx, r.URL.Path, func(ctx context.Context) error {
			propagation.InjectHTTP(ctx, Propagators, w.Header())
			hndl.ServeHTTP(w, r.WithContext(ctx))
			return nil
		})
	})
}

func GlobalTracer() trace.Tracer { return global.Tracer("github.com/tgulacsi/go/otel") }
func FromContext(ctx context.Context) trace.Tracer {
	if tracer, ok := ctx.Value(ckTracer).(trace.Tracer); ok {
		return tracer
	}
	return GlobalTracer()
}

type ctxKey string

const ckTracer = ctxKey("tracer")

type logfmtEncoder struct {
	id label.EncoderID
}

func NewExportPipeline(Log func(...interface{}) error) (trace.Provider, error) {
	exporter := LogExporter{Log: Log}
	return sdktrace.NewProvider(sdktrace.WithSyncer(exporter))
}

type LogExporter struct {
	Log func(...interface{}) error
}

// ExportSpans writes SpanData in json format to stdout.
func (e LogExporter) ExportSpans(ctx context.Context, data []*setrace.SpanData) {
	for _, d := range data {
		e.ExportSpan(ctx, d)
	}
}

// ExportSpan writes a SpanData in json format to stdout.
func (e LogExporter) ExportSpan(ctx context.Context, data *setrace.SpanData) {
	/*
	   type SpanData struct {
	   	SpanContext  apitrace.SpanContext
	   	ParentSpanID apitrace.SpanID
	   	SpanKind     apitrace.SpanKind
	   	Name         string
	   	StartTime    time.Time
	   	// The wall clock time of EndTime will be adjusted to always be offset
	   	// from StartTime by the duration of the span.
	   	EndTime                  time.Time
	   	Attributes               []kv.KeyValue
	   	MessageEvents            []Event
	   	Links                    []apitrace.Link
	   	StatusCode               codes.Code
	   	StatusMessage            string
	   	HasRemoteParent          bool
	   	DroppedAttributeCount    int
	   	DroppedMessageEventCount int
	   	DroppedLinkCount         int

	   	// ChildSpanCount holds the number of child span created for this span.
	   	ChildSpanCount int

	   	// Resource contains attributes representing an entity that produced this span.
	   	Resource *resource.Resource

	   	// InstrumentationLibrary defines the instrumentation library used to
	   	// providing instrumentation.
	   	InstrumentationLibrary instrumentation.Library
	   }
	*/
	e.Log("msg", "trace", "parent", data.ParentSpanID, "kind", data.SpanKind, "name", data.Name,
		"spanID", data.SpanContext.SpanID, "traceID", data.SpanContext.TraceID, "start", data.StartTime, "end", data.EndTime,
		"attrs", data.Attributes, "events", data.MessageEvents, "links", data.Links,
		"statusCode", data.StatusCode, "statusMsg", data.StatusMessage,
	)
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

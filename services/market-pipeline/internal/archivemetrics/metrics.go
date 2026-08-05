package archivemetrics

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300}

var outcomes = []string{"created", "duplicate", "quarantined", "error"}

type histogram struct {
	counts []uint64
	count  uint64
	sum    float64
}

// Recorder owns bounded-cardinality archiver metrics. It deliberately has no
// instrument, event ID, tenant, account, or portfolio labels.
type Recorder struct {
	mu         sync.Mutex
	scope      string
	events     map[string]uint64
	processing histogram
	durable    histogram
}

// New creates a recorder for one fixed public pipeline scope.
func New(scope string) (*Recorder, error) {
	if scope != "baseline" && scope != "scale" {
		return nil, errors.New(`metrics scope must be "baseline" or "scale"`)
	}
	return &Recorder{
		scope:  scope,
		events: map[string]uint64{},
		processing: histogram{
			counts: make([]uint64, len(buckets)+1),
		},
		durable: histogram{
			counts: make([]uint64, len(buckets)+1),
		},
	}, nil
}

// Observe records one terminal processing outcome and both processing and
// Kafka-to-durable-effect latency.
func (r *Recorder) Observe(outcome string, processing, durable time.Duration) error {
	if !validOutcome(outcome) {
		return fmt.Errorf("unsupported metrics outcome %q", outcome)
	}
	if processing < 0 || durable < 0 {
		return errors.New("metric durations cannot be negative")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events[outcome]++
	observe(&r.processing, processing.Seconds())
	observe(&r.durable, durable.Seconds())
	return nil
}

// Handler exposes the Prometheus text format without adding a metrics SDK to
// the small scratch image.
func (r *Recorder) Handler() http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/metrics" {
			http.NotFound(response, request)
			return
		}
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		response.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		r.mu.Lock()
		defer r.mu.Unlock()
		var output strings.Builder
		output.WriteString("# HELP market_archiver_events_total Terminal event processing outcomes.\n")
		output.WriteString("# TYPE market_archiver_events_total counter\n")
		for _, outcome := range outcomes {
			fmt.Fprintf(
				&output,
				"market_archiver_events_total{scope=%q,outcome=%q} %d\n",
				r.scope,
				outcome,
				r.events[outcome],
			)
		}
		writeHistogram(&output, "market_archiver_processing_duration_seconds", r.scope, r.processing)
		writeHistogram(&output, "market_archiver_durable_latency_seconds", r.scope, r.durable)
		_, _ = response.Write([]byte(output.String()))
	})
}

func validOutcome(candidate string) bool {
	for _, outcome := range outcomes {
		if candidate == outcome {
			return true
		}
	}
	return false
}

func observe(histogram *histogram, value float64) {
	for index, upperBound := range buckets {
		if value <= upperBound {
			histogram.counts[index]++
		}
	}
	histogram.counts[len(buckets)]++
	histogram.count++
	histogram.sum += value
}

func writeHistogram(output *strings.Builder, name, scope string, histogram histogram) {
	fmt.Fprintf(output, "# HELP %s Observed archiver latency.\n", name)
	fmt.Fprintf(output, "# TYPE %s histogram\n", name)
	for index, upperBound := range buckets {
		fmt.Fprintf(
			output,
			"%s_bucket{scope=%q,le=%q} %d\n",
			name,
			scope,
			strconv.FormatFloat(upperBound, 'g', -1, 64),
			histogram.counts[index],
		)
	}
	fmt.Fprintf(output, "%s_bucket{scope=%q,le=\"+Inf\"} %d\n", name, scope, histogram.counts[len(buckets)])
	fmt.Fprintf(output, "%s_sum{scope=%q} %s\n", name, scope, strconv.FormatFloat(histogram.sum, 'g', -1, 64))
	fmt.Fprintf(output, "%s_count{scope=%q} %d\n", name, scope, histogram.count)
}

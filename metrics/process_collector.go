package metrics

import (
	"errors"
	"github.com/prometheus/client_golang/prometheus"
	"os"
)

type processCollector struct {
	collectFn       func(chan<- prometheus.Metric)
	pidFn           func() (int, error)
	reportErrors    bool
	cpuTotal        *prometheus.Desc
	openFDs, maxFDs *prometheus.Desc
	vsize, maxVsize *prometheus.Desc
	rss             *prometheus.Desc
	startTime       *prometheus.Desc
	LabelsValue     []string
}

// ProcessCollectorOpts defines the behavior of a process metrics collector
// created with NewProcessCollector.
type ServiceProcessCollectorOpts struct {
	LabelsKey   []string
	LabelsValue []string
	// PidFn returns the PID of the process the collector collects metrics
	// for. It is called upon each collection. By default, the PID of the
	// current process is used, as determined on construction time by
	// calling os.Getpid().
	PidFn func() (int, error)
	// If non-empty, each of the collected metrics is prefixed by the
	// provided string and an underscore ("_").
	Namespace string
	// If true, any error encountered during collection is reported as an
	// invalid metric (see NewInvalidMetric). Otherwise, errors are ignored
	// and the collected metrics will be incomplete. (Possibly, no metrics
	// will be collected at all.) While that's usually not desired, it is
	// appropriate for the common "mix-in" of process metrics, where process
	// metrics are nice to have, but failing to collect them should not
	// disrupt the collection of the remaining metrics.
	ReportErrors bool
}

// NewProcessCollector returns a collector which exports the current state of
// process metrics including CPU, memory and file descriptor usage as well as
// the process start time. The detailed behavior is defined by the provided
// ProcessCollectorOpts. The zero value of ProcessCollectorOpts creates a
// collector for the current process with an empty namespace string and no error
// reporting.
//
// The collector only works on operating systems with a Linux-style proc
// filesystem and on Microsoft Windows. On other operating systems, it will not
// collect any metrics.
func NewServiceProcessCollector(opts ServiceProcessCollectorOpts) prometheus.Collector {
	ns := ""
	if len(opts.Namespace) > 0 {
		ns = opts.Namespace + "_"
	}

	c := &processCollector{
		reportErrors: opts.ReportErrors,
		cpuTotal: prometheus.NewDesc(
			ns+"process_cpu_seconds_total",
			"Total user and system CPU time spent in seconds.",
			opts.LabelsKey, nil,
		),
		openFDs: prometheus.NewDesc(
			ns+"process_open_fds",
			"Number of open file descriptors.",
			opts.LabelsKey, nil,
		),
		maxFDs: prometheus.NewDesc(
			ns+"process_max_fds",
			"Maximum number of open file descriptors.",
			opts.LabelsKey, nil,
		),
		vsize: prometheus.NewDesc(
			ns+"process_virtual_memory_bytes",
			"Virtual memory size in bytes.",
			opts.LabelsKey, nil,
		),
		maxVsize: prometheus.NewDesc(
			ns+"process_virtual_memory_max_bytes",
			"Maximum amount of virtual memory available in bytes.",
			opts.LabelsKey, nil,
		),
		rss: prometheus.NewDesc(
			ns+"process_resident_memory_bytes",
			"Resident memory size in bytes.",
			opts.LabelsKey, nil,
		),
		startTime: prometheus.NewDesc(
			ns+"process_start_time_seconds",
			"Start time of the process since unix epoch in seconds.",
			opts.LabelsKey, nil,
		),
		LabelsValue: opts.LabelsValue,
	}

	if opts.PidFn == nil {
		pid := os.Getpid()
		c.pidFn = func() (int, error) { return pid, nil }
	} else {
		c.pidFn = opts.PidFn
	}

	// Set up process metric collection if supported by the runtime.
	if canCollectProcess() {
		c.collectFn = c.processCollect
	} else {
		c.collectFn = func(ch chan<- prometheus.Metric) {
			c.reportError(ch, nil, errors.New("process metrics not supported on this platform"))
		}
	}

	return c
}

// Describe returns all descriptions of the collector.
func (c *processCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.cpuTotal
	ch <- c.openFDs
	ch <- c.maxFDs
	ch <- c.vsize
	ch <- c.maxVsize
	ch <- c.rss
	ch <- c.startTime
}

// Collect returns the current state of all metrics of the collector.
func (c *processCollector) Collect(ch chan<- prometheus.Metric) {
	c.collectFn(ch)
}

func (c *processCollector) reportError(ch chan<- prometheus.Metric, desc *prometheus.Desc, err error) {
	if !c.reportErrors {
		return
	}
	if desc == nil {
		desc = prometheus.NewInvalidDesc(err)
	}
	ch <- prometheus.NewInvalidMetric(desc, err)
}

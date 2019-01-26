package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

const (
	namespace = "resque"
)

var (
	redisNamespace = flag.String(
		"redis.namespace",
		"resque",
		"Namespace used by Resque to prefix all its Redis keys.",
	)
	redisURL = flag.String(
		"redis.url",
		"redis://localhost:6379",
		"URL to the Redis backing the Resque.",
	)
	printVersion = flag.Bool(
		"version",
		false,
		"Print version information.",
	)
	listenAddress = flag.String(
		"web.listen-address",
		":9447",
		"Address to listen on for web interface and telemetry.",
	)
	metricPath = flag.String(
		"web.telemetry-path",
		"/metrics",
		"Path under which to expose metrics.",
	)
)

var (
	failedJobExecutionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "failed_job_executions_total"),
		"Total number of failed job executions.",
		nil, nil,
	)
	jobExecutionsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "job_executions_total"),
		"Total number of job executions.",
		nil, nil,
	)
	jobsInFailedQueueDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "jobs_in_failed_queue"),
		"Number of jobs in a failed queue.",
		[]string{"queue"}, nil,
	)
	jobsInQueueDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "jobs_in_queue"),
		"Number of jobs in a queue.",
		[]string{"queue"}, nil,
	)
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "scrape_duration_seconds"),
		"Time this scrape of resque metrics took.",
		nil, nil,
	)
	upDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Whether this scrape of resque metrics was successful.",
		nil, nil,
	)
	workersDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "workers"),
		"Number of workers.",
		nil, nil,
	)
	workingWorkersDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "working_workers"),
		"Number of working workers.",
		nil, nil,
	)
)

// Exporter collects Resque metrics. It implements prometheus.Collector.
type Exporter struct {
	redisClient    *redis.Client
	redisNamespace string

	failedScrapes prometheus.Counter
	scrapes       prometheus.Counter
}

// NewExporter returns a new Resque exporter.
func NewExporter(redisURL, redisNamespace string) (*Exporter, error) {
	redisClient, err := newRedisClient(redisURL)
	if err != nil {
		return nil, err
	}

	return &Exporter{
		redisClient:    redisClient,
		redisNamespace: redisNamespace,
		failedScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "failed_scrapes_total",
			Help:      "Total number of failed scrapes.",
		}),
		scrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scrapes_total",
			Help:      "Total number of scrapes.",
		}),
	}, nil
}

func newRedisClient(redisURL string) (*redis.Client, error) {
	var options redis.Options

	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "redis" || u.Scheme == "tcp" {
		options.Network = "tcp"
		options.Addr = net.JoinHostPort(u.Hostname(), u.Port())
		if len(u.Path) > 1 {
			if db, err := strconv.Atoi(u.Path[1:]); err == nil {
				options.DB = db
			}
		}
	} else if u.Scheme == "unix" {
		options.Network = "unix"
		options.Addr = u.Path
	} else {
		return nil, fmt.Errorf("unknown URL scheme: %s", u.Scheme)
	}

	if password, ok := u.User.Password(); ok {
		options.Password = password
	}

	return redis.NewClient(&options), nil
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- failedJobExecutionsDesc
	ch <- jobExecutionsDesc
	ch <- jobsInFailedQueueDesc
	ch <- jobsInQueueDesc
	ch <- scrapeDurationDesc
	ch <- upDesc
	ch <- workersDesc
	ch <- workingWorkersDesc

	ch <- e.failedScrapes.Desc()
	ch <- e.scrapes.Desc()
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	if err := e.scrape(ch); err != nil {
		e.failedScrapes.Inc()
		log.Error(err)
		ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 0)
	} else {
		ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 1)
	}

	ch <- e.failedScrapes
	ch <- e.scrapes
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) error {
	e.scrapes.Inc()

	defer func(start time.Time) {
		ch <- prometheus.MustNewConstMetric(
			scrapeDurationDesc,
			prometheus.GaugeValue,
			float64(time.Since(start).Seconds()))
	}(time.Now())

	executions, err := e.redisClient.Get(e.redisKey("stat:processed")).Float64()
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(jobExecutionsDesc, prometheus.CounterValue, executions)

	failedExecutions, err := e.redisClient.Get(e.redisKey("stat:failed")).Float64()
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(failedJobExecutionsDesc, prometheus.CounterValue, failedExecutions)

	queues, err := e.redisClient.SMembers(e.redisKey("queues")).Result()
	if err != nil {
		return err
	}

	for _, queue := range queues {
		jobs, err := e.redisClient.LLen(e.redisKey("queue", queue)).Result()
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(jobsInQueueDesc, prometheus.GaugeValue, float64(jobs), queue)
	}

	failedQueues, err := e.redisClient.SMembers(e.redisKey("failed_queues")).Result()
	if err != nil {
		return err
	}

	if len(failedQueues) == 0 {
		exists, err := e.redisClient.Exists(e.redisKey("failed")).Result()
		if err != nil {
			return err
		}
		if exists == 1 {
			failedQueues = []string{"failed"}
		}
	}

	for _, queue := range failedQueues {
		jobs, err := e.redisClient.LLen(e.redisKey(queue)).Result()
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(jobsInFailedQueueDesc, prometheus.GaugeValue, float64(jobs), queue)
	}

	workers, err := e.redisClient.SMembers(e.redisKey("workers")).Result()
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(workersDesc, prometheus.GaugeValue, float64(len(workers)))

	var workingWorkers int
	for _, worker := range workers {
		exists, err := e.redisClient.Exists(e.redisKey("worker", worker)).Result()
		if err != nil {
			return err
		}
		if exists == 1 {
			workingWorkers++
		}
	}
	ch <- prometheus.MustNewConstMetric(workingWorkersDesc, prometheus.GaugeValue, float64(workingWorkers))

	return nil
}

func (e *Exporter) redisKey(a ...string) string {
	return e.redisNamespace + ":" + strings.Join(a, ":")
}

func init() {
	prometheus.MustRegister(version.NewCollector("resque-exporter"))
}

func main() {
	flag.Parse()

	if *printVersion {
		fmt.Println(version.Print("resque-exporter"))
		return
	}

	log.Infoln("Starting resque-exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	if u := os.Getenv("REDIS_URL"); len(u) > 0 {
		*redisURL = u
	}

	exporter, err := NewExporter(*redisURL, *redisNamespace)
	if err != nil {
		log.Fatal(err)
	}
	prometheus.MustRegister(exporter)

	http.Handle(*metricPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
<head><title>Resque Exporter</title></head>
<body>
<h1>Resque Exporter</h1>
<p><a href='` + *metricPath + `'>Metrics</a></p>
</body>
</html>
`))
	})

	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

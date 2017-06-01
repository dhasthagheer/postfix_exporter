package main

import (
	"flag"
	"net/http"
        "os/exec"
	"sync"
        "strings"
        "strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/log"
)

const (
	namespace = "postfix" // For Prometheus metrics.
)

var (
	metricsPath  = flag.String("telemetry.endpoint", "/metrics", "Path under which to expose metrics.")
)

// Exporter collects postfix stats from machine of a specified user and exports them using
// the prometheus metrics package.
type Exporter struct {
	mutex          sync.RWMutex
        totalQ         prometheus.Gauge
        incomingQ      prometheus.Gauge
        activeQ        prometheus.Gauge
        maildropQ      prometheus.Gauge
        deferredQ      prometheus.Gauge
        holdQ          prometheus.Gauge
        bounceQ        prometheus.Gauge
}

// NewPostfixExporter returns an initialized Exporter.
func NewPostfixExporter() *Exporter {
	return &Exporter{
          totalQ: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "total_queue_length",
			Help:      "length of mail queue",
		}),
          incomingQ: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "incoming_queue_length",
                        Help:      "length of incoming mail queue",
                }),
          activeQ: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "active_queue_length",
                        Help:      "length of active mail queue",
                }),
          maildropQ: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "maildrop_queue_length",
                        Help:      "length of maildrop queue",
                }),
          deferredQ: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "deferred_queue_length",
                        Help:      "length of deferred mail queue",
                }),
          holdQ: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "hold_queue_length",
                        Help:      "length of hold mail queue",
                }),
          bounceQ: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "bounce_queue_length",
                        Help:      "length of bounce mail queue",
                }),

	}
}

// Describe describes all the metrics ever exported by the postfix exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
    e.totalQ.Describe(ch)
    e.incomingQ.Describe(ch)
    e.activeQ.Describe(ch)
    e.maildropQ.Describe(ch)
    e.deferredQ.Describe(ch)
    e.holdQ.Describe(ch)
    e.bounceQ.Describe(ch)
}

func getPostfixQueueLength() string {
    cmd := "/usr/sbin/postqueue -p | tail -n1 | awk '{print $5}'"
    out, _ := exec.Command("bash", "-c", cmd).Output()
    len := string(out)
    len = strings.TrimSpace(len)
    return len
}

func getQueueDir() string{
    cmd := "postconf -h queue_directory"
    out,_ := exec.Command("bash", "-c", cmd).Output()
    dir := string(out)
    dir = strings.TrimSpace(dir)
    return dir
}

func getQueueLength(qname string, qdir string) string{
    cmd := "sudo find "+qdir+"/"+qname+" -type f -print | wc -l | awk '{print $1}'"
    out,_ := exec.Command("bash", "-c", cmd).Output()
    qlen := string(out)
    qlen = strings.TrimSpace(qlen)
    return qlen
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) error {
    queue_length, _ := strconv.ParseFloat(getPostfixQueueLength(), 64)
    e.totalQ.Set(float64(queue_length))
    queue_dir := getQueueDir()
    
    incoming_queue, _ := strconv.ParseFloat(getQueueLength("incoming", queue_dir), 64)
    e.incomingQ.Set(float64(incoming_queue))
   
    active_queue, _ := strconv.ParseFloat(getQueueLength("active", queue_dir), 64)
    e.activeQ.Set(float64(active_queue))

    maildrop_queue, _ := strconv.ParseFloat(getQueueLength("maildrop", queue_dir), 64)
    e.maildropQ.Set(float64(maildrop_queue))
   
    deferred_queue, _ := strconv.ParseFloat(getQueueLength("deferred", queue_dir), 64)
    e.deferredQ.Set(float64(deferred_queue))

    hold_queue, _ := strconv.ParseFloat(getQueueLength("hold", queue_dir), 64)
    e.holdQ.Set(float64(hold_queue))
   
    bounce_queue, _ := strconv.ParseFloat(getQueueLength("bounce", queue_dir), 64)
    e.bounceQ.Set(float64(bounce_queue))

    return nil
}

// Collect fetches the stats of a user and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()
        if err := e.scrape(ch); err != nil {
		log.Printf("Error scraping postfix: %s", err)
	}
        e.totalQ.Collect(ch)
        e.incomingQ.Collect(ch)
        e.activeQ.Collect(ch)
        e.maildropQ.Collect(ch)
        e.deferredQ.Collect(ch)
        e.holdQ.Collect(ch)
        e.bounceQ.Collect(ch)
	return
}

func main() {
	flag.Parse()

	exporter := NewPostfixExporter()
	prometheus.MustRegister(exporter)
	http.Handle(*metricsPath, prometheus.Handler())
        http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	    w.Write([]byte(`<html>
                <head><title>Postfix exporter</title></head>
                <body>
                   <h1>Postfix exporter</h1>
                   <p><a href='` + *metricsPath + `'>Metrics</a></p>
                   </body>
                </html>
              `))
	})
	log.Fatal(http.ListenAndServe(":0", nil))
}

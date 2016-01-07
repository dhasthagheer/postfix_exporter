package main

import (
	"flag"
	"net/http"
        "os/exec"
	"sync"
        "strings"
        "strconv"
        "time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/log"
)

const (
	namespace = "postfix" // For Prometheus metrics.
)

var (
	listenAddress = flag.String("telemetry.address", ":9115", "Address on which to expose metrics.")
	metricsPath  = flag.String("telemetry.endpoint", "/metrics", "Path under which to expose metrics.")
        pflogsummEnabled = flag.String("pflogsumm", "no", "Collect and expose pflogsumm statistics")
        pflogsummLog = flag.String("pflogsummLog", "", "Mail logfile to parse for pflogsumm (non systemd mode)")
        systemdEnabled = flag.String("systemd", "no", "Collect and expose pflogsumm statistics")
        pflogsummInterval = flag.Int64("pflogsummInterval", 60, "Interval to update pflogsumm statistics (re-run pflogsumm)")
        lastPostfixLogsumm int64 = 0
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
        received       prometheus.Gauge
        delivered      prometheus.Gauge
        forwarded      prometheus.Gauge
        deferred       prometheus.Gauge
        bounced        prometheus.Gauge
        rejected       prometheus.Gauge
        held           prometheus.Gauge
        discarded      prometheus.Gauge
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
          received: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "received_mails",
                        Help:      "number received mails",
                }),
          delivered: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "delivered_mails",
                        Help:      "number delivered mails",
                }),
          forwarded: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "forwarded_mails",
                        Help:      "number forwarded mails",
                }),
          deferred: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "deferred_mails",
                        Help:      "number deferred mails",
                }),
          bounced: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "bounced_mails",
                        Help:      "number bounced mails",
                }),
          rejected: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "rejected_mails",
                        Help:      "number rejected mails",
                }),
          held: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "held_mails",
                        Help:      "number held mails",
                }),
          discarded: prometheus.NewGauge(prometheus.GaugeOpts{
                        Namespace: namespace,
                        Name:      "discarded_mails",
                        Help:      "number discarded mails",
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
    e.received.Describe(ch)
    e.delivered.Describe(ch)
    e.forwarded.Describe(ch)
    e.deferred.Describe(ch)
    e.bounced.Describe(ch)
    e.rejected.Describe(ch)
    e.held.Describe(ch)
    e.discarded.Describe(ch)
}

func parsePostfixLogfile() {
    var cmd string

    if (*pflogsummEnabled == "no") { 
        return
    }

    if ((lastPostfixLogsumm + *pflogsummInterval) > time.Now().Unix()) {
        return
    }


    if (*systemdEnabled == "yes") {
        cmd = "journalctl -u postfix.service --since today | pflogsumm --smtpd_stats | head -n 15 > /tmp/postfix_exporter.stats;"
    } else {
        cmd = "pflogsumm --smtpd_stats -d today "+*pflogsummLog+" | head -n 15 > /tmp/postfix_exporter.stats;"
    }

    log.Infof("Running: "+cmd)
    exec.Command("bash", "-c", cmd).Output()
    lastPostfixLogsumm = time.Now().Unix();
}

func getPostfixStat(stat string) string {
    parsePostfixLogfile()
    cmd := "grep '"+stat+"' /tmp/postfix_exporter.stats | awk '{print $1}'"
    out, _ := exec.Command("bash", "-c", cmd).Output()
    num := string(out)
    num = strings.TrimSpace(num)
    return num
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

    if (*pflogsummEnabled == "yes") {
        received, _ := strconv.ParseFloat(getPostfixStat("received"), 64)
        e.received.Set(float64(received))

        delivered, _ := strconv.ParseFloat(getPostfixStat("delivered"), 64)
        e.delivered.Set(float64(delivered))

        forwarded, _ := strconv.ParseFloat(getPostfixStat("forwarded"), 64)
        e.forwarded.Set(float64(forwarded))

        deferred, _ := strconv.ParseFloat(getPostfixStat("deferred"), 64)
        e.deferred.Set(float64(deferred))

        bounced, _ := strconv.ParseFloat(getPostfixStat("bounced"), 64)
        e.bounced.Set(float64(bounced))

        rejected, _ := strconv.ParseFloat(getPostfixStat("rejected"), 64)
        e.rejected.Set(float64(rejected))

        held, _ := strconv.ParseFloat(getPostfixStat("held"), 64)
        e.held.Set(float64(held))

        discarded, _ := strconv.ParseFloat(getPostfixStat("discarded"), 64)
        e.discarded.Set(float64(discarded))
    }

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
        if (*pflogsummEnabled == "yes") {
            e.received.Collect(ch)
            e.delivered.Collect(ch)
            e.forwarded.Collect(ch)
            e.deferred.Collect(ch)
            e.bounced.Collect(ch)
            e.rejected.Collect(ch)
            e.held.Collect(ch)
            e.discarded.Collect(ch)
        }
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
        if (*systemdEnabled == "no" && *pflogsummEnabled == "yes" && *pflogsummLog == "") {
           log.Fatal("Systemd disabled, pflogsumm enabled but no logfile given via -pflogsummLog!")
        }
	log.Infof("Starting Server: %s", *listenAddress)
	log.Infof("systemd enabled: %s", *systemdEnabled)
	log.Infof("pflogsumm enabled: %s", *pflogsummEnabled)
	log.Infof("pflogsumm interval: %d", *pflogsummInterval)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

package main

// A lot of the code is inspired from https://github.com/prometheus/blackbox_exporter/

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
)

func main() {
	log.Infoln("Beginning to serve on port :9876")

	// This is just a demo dummyMetric that is initialised and never changed
	var dummyMetric = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "dummy_metric", Help: "Shows whether a dummy has occurred in our cluster"})
	prometheus.MustRegister(dummyMetric)
	dummyMetric.Set(0)

	http.Handle("/metrics", promhttp.Handler()) // Do we really want this ?
	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		probeHandler(w, r)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><head><title>PCSensor Exporter</title></head><body><h1>PCSensor Exporter</h1>"))
		w.Write([]byte("<form action='/probe'>Target IP : <input name='target'>&nbsp;<input type='submit'></form>"))
		w.Write([]byte("</body></html>"))
	})

	log.Fatalln(http.ListenAndServe(":9876", nil))
}

func probeHandler(w http.ResponseWriter, r *http.Request) {

	timeoutSeconds, err := getTimeout(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse timeout from Prometheus header: %s", err), http.StatusInternalServerError)
		return
	}

	params := r.URL.Query()
	target := params.Get("target")
	if target == "" {
		http.Error(w, "Target parameter is missing", http.StatusBadRequest)
		return
	}

	sensorcount, err := strconv.Atoi(params.Get("count"))
	if err != nil {
		sensorcount=2
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()
	r = r.WithContext(ctx)

	probeSuccessGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_success",
		Help: "Displays whether or not the probe was a success",
	})
	probeDurationGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_duration_seconds",
		Help: "Returns how long the probe took to complete in seconds",
	})

	start := time.Now()
	registry := prometheus.NewRegistry()
	registry.MustRegister(probeSuccessGauge)
	registry.MustRegister(probeDurationGauge)
	success := probe(ctx, target, sensorcount, registry)
	duration := time.Since(start).Seconds()
	probeDurationGauge.Set(duration)
	if success {
		probeSuccessGauge.Set(1)
		log.Debugf("Probe of %s succeeded in %0.6f sec(s)\n", target, duration)
	} else {
		log.Errorf("Probe of %s failed in %0.6f sec(s)\n", target, duration)
	}

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

func getTimeout(r *http.Request) (timeoutSeconds float64, err error) {
	// If a timeout is configured via the Prometheus header, add it to the request.
	if v := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
		var err error
		timeoutSeconds, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, err
		}
	}
	if timeoutSeconds == 0 {
		timeoutSeconds = 60
	}
	return timeoutSeconds, nil
}

func probe(ctx context.Context, target string, sensorcount int, registry *prometheus.Registry) (success bool) {
	log.Debugf("Probing target %s\n", target)

	tempGaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "probe_pcsensors_temperature_celcius",
		Help: "Temperature detected by pcsensors probe in celcius",
	}, []string{"probe"})
	registry.MustRegister(tempGaugeVec)
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/", target), nil)
	if err != nil {
		log.Errorln("Error making request:", err)
		return false
	}
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorln("Error doing request:", err)
		return false
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln("Error reading response body:", err)
	}
	for sensor := 1;  sensor<=sensorcount; sensor++ {
		re := regexp.MustCompile(fmt.Sprintf("T%0d<p>.+?(\\d+\\.\\d+)",sensor))
		match := re.FindStringSubmatch(string(body))
		if match != nil {
			f, err := strconv.ParseFloat(match[1], 64)
			if err != nil { 
				log.Errorln("Error converting ",match[1], " to float -", err)
				return false
				
			}  else {
				sensorstring := fmt.Sprintf("T%0d",sensor)
				tempGaugeVec.WithLabelValues(sensorstring)
				tempGaugeVec.WithLabelValues(sensorstring).Add(f)
			}
		}
	}

	return true
}

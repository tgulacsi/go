package main

import (
	"bufio"
	"bytes"
	"flag"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	ui "github.com/gizak/termui"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/retrieval"
)

//go:generate rm -rf $GOPATH/src/github.com/prometheus/prometheus/vendor/github.com/prometheus/common/model

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	flagAddress := flag.String("addr", ":9100", "Prometheus metrics address")
	flagInterval := flag.Duration("interval", time.Second, "scrape interval")
	flag.Parse()

	if !strings.Contains(*flagAddress, "/") {
		*flagAddress += "/metrics"
	}
	if strings.HasPrefix(*flagAddress, ":") {
		*flagAddress = "http://localhost" + *flagAddress
	}
	u, err := url.Parse(*flagAddress)
	if err != nil {
		return errors.Wrap(err, *flagAddress)
	}

	pattern := regexp.MustCompile(strings.Join(flag.Args(), "|"))

	cfg := &config.Config{
		GlobalConfig: config.GlobalConfig{
			ScrapeInterval:     model.Duration(*flagInterval),
			ScrapeTimeout:      model.Duration(*flagInterval * 9 / 10),
			EvaluationInterval: model.Duration(15 * time.Second),
			ExternalLabels:     model.LabelSet{"monitor": "logterm"},
		},
	}
	cfg.ScrapeConfigs = []*config.ScrapeConfig{
		&config.ScrapeConfig{JobName: "stdin", HonorLabels: true,
			ScrapeInterval: cfg.GlobalConfig.ScrapeInterval,
			ScrapeTimeout:  cfg.GlobalConfig.ScrapeTimeout,
			MetricsPath:    u.Path, Scheme: u.Scheme,
			ServiceDiscoveryConfig: config.ServiceDiscoveryConfig{
				StaticConfigs: []*config.TargetGroup{
					&config.TargetGroup{
						Targets: []model.LabelSet{model.LabelSet{
							"__address__": model.LabelValue(u.Hostname() + ":" + u.Port()),
						}},
					},
				},
			},
		},
	}
	samples := &memoryStorage{
		URL:     u,
		types:   make(map[string]metricType),
		samples: make(map[model.Fingerprint]gauge),
		filter:  func(m model.Metric) bool { return pattern.FindString(m.String()) != "" },
	}
	mngr := retrieval.NewTargetManager(samples)
	mngr.ApplyConfig(cfg)
	defer mngr.Stop()
	go mngr.Run()

	if true {
		time.Sleep(1 * time.Second)
		log.Println(mngr.Targets())
		select {}
	} else {
		if err := ui.Init(); err != nil {
			panic(err)
		}
		defer ui.Close()

		ui.Handle("/sys/kbd/q", func(ui.Event) {
			// press q to quit
			mngr.Stop()
			ui.StopLoop()
		})

		ui.Loop()
	}
	return nil
}

type metricType uint8

const (
	Untyped = metricType(iota)
	Counter
	Gauge
	Histogram
	Summary
)

type gauge struct {
	*model.Sample
	*ui.Gauge
}
type memoryStorage struct {
	*url.URL
	*http.Client
	types   map[string]metricType
	samples map[model.Fingerprint]gauge
	filter  func(model.Metric) bool
}

func (a *memoryStorage) NeedsThrottling() bool { return false }

func (a *memoryStorage) Append(sample *model.Sample) error {
	if !a.filter(sample.Metric) {
		return nil
	}
	log.Println(sample.Metric, sample.Value)
	k := sample.Metric.Fingerprint()
	if g, ok := a.samples[k]; ok {
		g.Sample = sample
		g.Percent = int(sample.Value * 1000)
		ui.Render(g)
		return nil
	}
	s := sample.Metric.String()
	if i := strings.IndexByte(s, '{'); i >= 0 {
		s = s[:i]
	}
	t, err := a.GetTypeOf(s)
	if err != nil {
		return err
	}
	log.Printf("%s=%d", s, t)

	g := gauge{
		Sample: sample,
		Gauge:  ui.NewGauge(),
	}
	g.Width = 100
	g.BorderLabel = g.Sample.String()
	ui.Render(g)
	a.samples[k] = g
	return nil
}

func (a *memoryStorage) GetTypeOf(s string) (metricType, error) {
	if t, ok := a.types[s]; ok {
		return t, nil
	}

	cl := a.Client
	if cl == nil {
		cl = http.DefaultClient
	}
	resp, err := cl.Get(a.URL.String())
	if err != nil {
		return Untyped, errors.Wrap(err, a.URL.String())
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if !bytes.HasPrefix(scanner.Bytes(), []byte("# TYPE ")) {
			continue
		}
		b := scanner.Bytes()[7:]
		i := bytes.IndexByte(b, ' ')
		if i < 0 {
			continue
		}
		t := Untyped
		switch string(b[i+1:]) {
		case "gauge":
			t = Gauge
		case "counter":
			t = Counter
		case "histogram":
			t = Histogram
		case "summary":
			t = Summary
		}
		a.types[string(b[:i])] = t
	}
	return a.types[s], scanner.Err()
}

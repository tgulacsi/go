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
		URL:        u,
		types:      make(map[string]metricType),
		gauges:     make(map[model.Fingerprint]*gauge),
		filter:     func(m model.Metric) bool { return pattern.FindString(m.String()) != "" },
		newUIGauge: func(_ string) func(int) { return func(_ int) {} },
	}
	mngr := retrieval.NewTargetManager(samples)
	mngr.ApplyConfig(cfg)
	defer mngr.Stop()
	go mngr.Run()

	if false {
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

		sGrp := ui.NewSparklines()
		sGrp.Height = 8 * 3
		sGrp.Width = ui.TermWidth()
		samples.newUIGauge = func(name string) func(int) {
			sl := ui.NewSparkline()
			sl.Title = name
			sl.Height = 2
			sl.Data = make([]int, sGrp.Width)
			sGrp.Add(sl)

			return func(p int) {
				copy(sl.Data[0:], sl.Data[1:])
				sl.Data[len(sl.Data)-1] = p
			}
		}

		go func() {
			for range time.Tick(*flagInterval) {
				ui.Render(sGrp)
			}
		}()

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
	Name       string
	Type       metricType
	Last       model.SampleValue
	SetPercent func(int)
}
type memoryStorage struct {
	*url.URL
	*http.Client
	types      map[string]metricType
	gauges     map[model.Fingerprint]*gauge
	filter     func(model.Metric) bool
	newUIGauge func(name string) (SetPercent func(int))
}

func (a *memoryStorage) NeedsThrottling() bool { return false }

func (a *memoryStorage) Append(sample *model.Sample) error {
	if !a.filter(sample.Metric) {
		return nil
	}
	k := sample.Metric.Fingerprint()
	g, ok := a.gauges[k]
	if !ok {
		full := sample.Metric.String()
		base := full
		if i := strings.IndexByte(base, '{'); i >= 0 {
			base = base[:i]
		}
		t, err := a.GetTypeOf(base)
		if err != nil {
			return err
		}

		g = &gauge{Name: full, Type: t, SetPercent: a.newUIGauge(full)}
		a.gauges[k] = g
	}

	v := sample.Value
	if g.Type == Counter || g.Type == Untyped {
		if g.Last == 0 {
			v = 0
		} else {
			v -= g.Last
		}
	}
	g.SetPercent(int(v))
	g.Last = sample.Value
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

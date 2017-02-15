package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	ui "github.com/jroimartin/gocui"
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
	flagInterval := flag.Duration("interval", 5*time.Second, "scrape interval")
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

	patternS := strings.Join(flag.Args(), "|")
	if patternS == "" {
		patternS = "."
	}
	pattern := regexp.MustCompile(patternS)

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
		URL:    u,
		types:  make(map[string]metricType),
		gauges: make(map[model.Fingerprint]*gauge),
		filter: func(m model.Metric) bool { return pattern.FindString(m.String()) != "" },
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
		g, err := ui.NewGui(ui.OutputNormal)
		if err != nil {
			panic(err)
		}
		defer g.Close()

		quit := func(g *ui.Gui, v *ui.View) error {
			go mngr.Stop()
			return ui.ErrQuit
		}

		layout := func(g *ui.Gui) error {
			maxX, maxY := g.Size()
			if v, err := g.SetView("metrics", 0, 0, maxX-1, maxY/2); err != nil {
				if err != ui.ErrUnknownView {
					return err
				}
				v.Title = "speed"
				v.Frame = true
			}
			if v, err := g.SetView("stdin", 0, maxY/2, maxX-1, maxY); err != nil {
				if err != ui.ErrUnknownView {
					return err
				}
				v.Title = "stdin"
				v.Frame = false
				v.Autoscroll = true
			}
			return nil
		}

		g.SetManagerFunc(layout)

		go func() {
			var linesMu sync.RWMutex
			lines := make([]string, 0, 128)
			go func() {
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					linesMu.Lock()
					lines = append(lines, scanner.Text())
					if cap(lines) == len(lines) {
						lines = lines[1:]
					}
					linesMu.Unlock()
				}
			}()

			for range time.Tick(time.Second) {
				g.Execute(func(g *ui.Gui) error {
					v, err := g.View("stdin")
					if err != nil {
						return err
					}
					v.Clear()
					linesMu.RLock()
					for _, line := range lines {
						v.Write([]byte(line))
						v.Write([]byte{'\n'})
					}
					linesMu.RUnlock()

					return nil
				})
			}
		}()

		go func() {
			for range time.Tick(*flagInterval) {
				g.Execute(func(g *ui.Gui) error {
					v, err := g.View("metrics")
					if err != nil {
						return err
					}
					v.Clear()
					samples.RLock()
					for _, g := range samples.gauges {
						g.RLock()
						fmt.Fprintf(v, "%s: %v\n", g.Name, g.SampleValue)
						g.RUnlock()
					}
					samples.RUnlock()

					return nil
				})
			}
		}()

		if err := g.SetKeybinding("", 'q', ui.ModNone, quit); err != nil {
			panic(err)
		}
		if err := g.SetKeybinding("", ui.KeyCtrlC, ui.ModNone, quit); err != nil {
			panic(err)
		}
		if err := g.MainLoop(); err != nil && err != ui.ErrQuit {
			panic(err)
		}
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
	sync.RWMutex
	Name string
	model.SampleValue
	Type metricType
}
type memoryStorage struct {
	*url.URL
	*http.Client
	sync.RWMutex
	types  map[string]metricType
	gauges map[model.Fingerprint]*gauge
	filter func(model.Metric) bool
}

func (a *memoryStorage) NeedsThrottling() bool { return false }

func (a *memoryStorage) Append(sample *model.Sample) error {
	if !a.filter(sample.Metric) {
		return nil
	}
	k := sample.Metric.Fingerprint()
	a.RLock()
	g, ok := a.gauges[k]
	a.RUnlock()
	if ok {
		g.Lock()
		g.SampleValue = sample.Value
		g.Unlock()
		return nil
	}
	full := sample.Metric.String()
	base := full
	if i := strings.IndexByte(base, '{'); i >= 0 {
		base = base[:i]
	}
	t, err := a.GetTypeOf(base)
	if err != nil {
		return err
	}

	g = &gauge{SampleValue: sample.Value, Type: t, Name: full}
	a.Lock()
	a.gauges[k] = g
	a.Unlock()

	return nil
}

func (a *memoryStorage) GetTypeOf(s string) (metricType, error) {
	a.RLock()
	t, ok := a.types[s]
	a.RUnlock()
	if ok {
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
	a.Lock()
	defer a.Unlock()
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

// Copyright 2017 Tamás Gulácsi
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	ui "github.com/jroimartin/gocui"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/retrieval"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/local"
)

//go:generate rm -rf $GOPATH/src/github.com/prometheus/prometheus/vendor/github.com/prometheus/common/model

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	flagAddress := fs.String("addr", ":9100", "Prometheus metrics address")
	flagInterval := fs.Duration("interval", 5*time.Second, "scrape interval")
	fs.Parse(os.Args[1:])

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

	storage, storCloser := local.NewTestStorage(log.New(os.Stderr, "[promstor] ", 0), 0)
	defer storCloser.Close()
	engine := promql.NewEngine(storage, &promql.EngineOptions{
		MaxConcurrentQueries: 8, Timeout: time.Duration(cfg.GlobalConfig.ScrapeTimeout),
	})

	samples := &memoryStorage{
		URL:     u,
		types:   make(map[string]metricType),
		gauges:  make(map[model.Fingerprint]*gauge),
		storage: storage,
	}
	queries := fs.Args()
	if len(queries) == 0 {
		queries = append(queries, "*")
	}
	mngr := retrieval.NewTargetManager(samples)
	mngr.ApplyConfig(cfg)
	defer mngr.Stop()
	go mngr.Run()

	if false {
		time.Sleep(1 * time.Second)
		log.Println(mngr.Targets())
		v := os.Stderr
		for t := range time.Tick(*flagInterval) {
			now := model.TimeFromUnix(t.Unix())
			ctx, cancel := context.WithDeadline(context.Background(), t.Add(*flagInterval-1))
			for _, qs := range queries {
				qry, err := engine.NewRangeQuery(qs, now-100, now, 1*time.Second)
				log.Printf("%q => %v/%v", qs, qry, err)
				if err != nil {
					fmt.Fprintf(v, "%q: %v\n", qs, err)
					return errors.Wrap(err, qs)
				}
				res := qry.Exec(ctx)
				log.Printf("res=%#v", res)
				if res.Err != nil {
					fmt.Fprintf(v, "%q: %v\n", qs, err)
					return errors.Wrap(err, qs)
				}
				fmt.Fprintf(v, "%s\n", res.Value.String())
			}
			cancel()
		}
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
			for t := range time.Tick(*flagInterval) {
				g.Execute(func(g *ui.Gui) error {
					v, err := g.View("metrics")
					if err != nil {
						return err
					}
					width, _ := v.Size()
					v.Clear()
					samples.RLock()
					if false {
						for _, g := range samples.gauges {
							g.RLock()
							fmt.Fprintf(v, "%s: %v\n", g.Name, g.SampleValue)
							g.RUnlock()
						}
					}
					samples.RUnlock()

					ctx, cancel := context.WithDeadline(context.Background(), t.Add(*flagInterval-1))
					defer cancel()

					now := model.TimeFromUnix(t.Unix())
					for _, qs := range queries {
						qry, err := engine.NewInstantQuery(qs, now)
						if err != nil {
							fmt.Fprintf(v, "%q: %v\n", qs, err)
							return errors.Wrap(err, qs)
						}
						res := qry.Exec(ctx)
						if res.Err != nil {
							fmt.Fprintf(v, "%q: %v\n", qs, err)
							return errors.Wrap(err, qs)
						}
						printQueryRes(
							wrapWriter{Writer: v, Width: width - 1},
							qry.Statement().String(),
							res.Value,
						)
					}

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
	types   map[string]metricType
	gauges  map[model.Fingerprint]*gauge
	storage storage.SampleAppender
}

func (a *memoryStorage) NeedsThrottling() bool { return false }

func (a *memoryStorage) Append(sample *model.Sample) error {
	if a.storage != nil {
		a.storage.Append(sample)
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

func printQueryRes(w io.Writer, name string, v model.Value) {
	if v == nil {
		return
	}
	switch v.Type() {
	case model.ValScalar:
		fmt.Fprintf(w, "%s: %v\n", name, v.(*model.Scalar).Value)
	case model.ValVector:
		vv := v.(model.Vector)
		sort.Sort(vecByName(vv))
		for _, s := range vv {
			fmt.Fprintf(w, "%s: %v\n", s.Metric.String(), s.Value)
		}
	case model.ValMatrix:
		m := v.(model.Matrix)
		sort.Sort(mtxByName(m))
		for _, ss := range m {
			nm := ss.Metric.String()
			for _, vv := range ss.Values {
				fmt.Fprintf(w, "%s: %v", nm, vv)
			}
		}
	default:
		fmt.Fprintf(w, "%s: %s\n", name, v.String())
	}
}

type wrapWriter struct {
	Width int
	io.Writer
}

func (w wrapWriter) Write(p []byte) (int, error) {
	if len(p) <= w.Width {
		return w.Writer.Write(p)
	}
	ew := &errWriter{Writer: w.Writer}
	for len(p) > w.Width {
		if i := bytes.LastIndexAny(p[:w.Width], "{},"); i > 1 {
			if p[i] != '{' {
				i++
			}
			ew.Write(p[:i])
			p = p[i:]
		}
		ew.Write([]byte{'\n', ' ', ' '})
	}
	ew.Write(p)
	return ew.N, ew.Err
}

type vecByName model.Vector

func (v vecByName) Len() int           { return len(v) }
func (v vecByName) Swap(i, j int)      { v[i], v[j] = v[j], v[i] }
func (v vecByName) Less(i, j int) bool { return v[i].Metric.String() < v[j].Metric.String() }

type mtxByName model.Matrix

func (m mtxByName) Len() int           { return len(m) }
func (m mtxByName) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m mtxByName) Less(i, j int) bool { return m[i].Metric.String() < m[j].Metric.String() }

type errWriter struct {
	io.Writer
	N   int
	Err error
}

func (w *errWriter) Write(p []byte) (int, error) {
	if w.Err != nil {
		return 0, w.Err
	}
	n, err := w.Writer.Write(p)
	w.N += n
	w.Err = err
	return n, err
}

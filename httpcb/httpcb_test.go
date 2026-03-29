// Copyright  2026 Tamás Gulácsi. All rights reserved.

package httpcb_test

import (
	jsonv1 "encoding/json"
	"testing"
	"time"

	jsonv2 "github.com/go-json-experiment/json"
	"github.com/google/go-cmp/cmp"
	"github.com/tgulacsi/go/httpcb"
)

func TestSettingsJSON(t *testing.T) {
	var want httpcb.Settings
	want.Name = "test"
	want.MaxRequests = 3
	want.BucketPeriod = time.Minute
	want.Timeout = 2 * time.Minute

	for name, m := range map[string]func(v any) ([]byte, error){
		"v1": func(v any) ([]byte, error) {
			s, _ := v.(httpcb.Settings)
			return jsonv1.Marshal(struct {
				Name                            string
				MaxRequests                     uint32
				Interval, BucketPeriod, Timeout time.Duration
			}{Name: s.Name,
				MaxRequests:  s.MaxRequests,
				Interval:     s.Interval,
				BucketPeriod: s.BucketPeriod,
				Timeout:      s.Timeout})
		},
		"v2": func(v any) ([]byte, error) { return jsonv2.Marshal(v) },
	} {
		t.Run(name, func(t *testing.T) {
			b, err := m(want)
			if err != nil {
				t.Fatal("marshal:", err)
			}
			t.Log(string(b))
			var got httpcb.Settings
			if err = jsonv1.Unmarshal(b, &got); err != nil {
				t.Fatal("unmarshal v1:", err)
			}
			if d := cmp.Diff(want, got); d != "" {
				t.Error("diff v1:", d)
			}

			if err = jsonv2.Unmarshal(b, &got); err != nil {
				t.Fatal("unmarshal v2:", err)
			}
			if d := cmp.Diff(want, got); d != "" {
				t.Error("diff v2:", d)
			}
		})
	}
}

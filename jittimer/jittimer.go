/*
  Copyright 2019 Tamás Gulácsi

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

// Package jittimer provides a ticker, a chan that sends time on a jittered interval.
package jittimer

import (
	"context"
	"math/rand"
	"time"
)

// NewTicker returns a chan time.Time that provides time.Time on a jittered regular interval.
//
// The underlying time.Timer will be closed when the context.Context is Done.
func NewTicker(ctx context.Context, d, stddev time.Duration) <-chan time.Time {
	duration := func() time.Duration {
		return d + time.Duration(rand.NormFloat64()*float64(stddev))
	}
	ch := make(chan time.Time)
	go func() {
		timer := time.NewTimer(duration())
		for {
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			case t := <-timer.C:
				select {
				case ch <- t:
					timer.Reset(duration())
				default:
				}
			}
		}
	}()
	return ch
}

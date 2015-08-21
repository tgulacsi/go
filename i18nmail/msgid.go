// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package i18nmail

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"
)

var (
	hostname    string
	getHostname sync.Once
)

// MakeMsgID creates a new, globally unique message ID, useable as
// a Message-ID as per RFC822/RFC2822.
func MakeMsgID() string {
	getHostname.Do(func() {
		var err error
		if hostname, err = os.Hostname(); err != nil {
			logger.Error("msg", "get hostname", "error", err)
			hostname = "localhost"
		}
	})
	now := time.Now()
	return fmt.Sprintf("<%d.%d.%d@%s>", now.Unix(), now.UnixNano(), rand.Int63(), hostname)
}

/*
Copyright 2015 Tamás Gulácsi

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

// Package osgroup contains functions for reading OS group names.
package osgroup

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	lastmod, lastcheck time.Time
	groups             map[int]string
	groupsMu           sync.RWMutex
)

const groupFile = "/etc/group"

// GroupName returns the name for the gid.
func GroupName(gid int) (string, error) {
	groupsMu.RLock()
	if groups == nil {
		groupsMu.RUnlock()
		groupsMu.Lock()
		defer groupsMu.Unlock()
		if groups != nil { // sy was faster
			name := groups[gid]
			return name, nil
		}
	} else {
		now := time.Now()
		if lastcheck.Add(1 * time.Second).After(now) { // fresh
			name := groups[gid]
			groupsMu.RUnlock()
			return name, nil
		}
		actcheck := lastcheck
		groupsMu.RUnlock()

		groupsMu.Lock()
		defer groupsMu.Unlock()
		if lastcheck != actcheck { // sy was faster
			return groups[gid], nil
		}
		fi, err := os.Stat(groupFile)
		if err != nil {
			return "", err
		}
		lastcheck = now
		if lastmod == fi.ModTime() { // no change
			return groups[gid], nil
		}
	}

	// need to reread
	if groups == nil {
		groups = make(map[int]string, 64)
	} else {
		for k := range groups {
			delete(groups, k)
		}
	}

	fh, err := os.Open(groupFile)
	if err != nil {
		return "", err
	}
	defer fh.Close()
	fi, err := fh.Stat()
	if err != nil {
		return "", err
	}
	lastcheck = time.Now()
	lastmod = fi.ModTime()

	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 4)
		id, err := strconv.Atoi(parts[2])
		if err != nil {
			log.Printf("cannot parse %q as group id from line %q: %v", parts[2], scanner.Text(), err)
		}
		if old, ok := groups[id]; ok {
			log.Printf("double entry %d: %q and %q?", id, old, parts[0])
			continue
		}
		groups[id] = parts[0]
	}

	return groups[gid], nil
}

// IsInsideDocker returns true iff we are inside a docker cgroup.
func IsInsideDocker() bool {
	b, err := ioutil.ReadFile("/proc/self/cgroup")
	if err != nil {
		return false
	}
	return bytes.Contains(b, []byte(":/docker/")) || bytes.Contains(b, []byte(":/lxc/"))
}

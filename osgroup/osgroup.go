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

// Package osgroup contains functions for reading OS group names.
package osgroup

import (
	"bufio"
	"io/ioutil"
	"log"
	"os"
	"os/user"
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

func getOrLookup(gid int) (string, error) {
	groupsMu.RLock()
	name := groups[gid]
	groupsMu.RUnlock()
	if name != "" {
		return name, nil
	}
	g, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		return "", err
	}
	name = g.Name
	groupsMu.Lock()
	groups[gid] = name
	groupsMu.Unlock()
	return name, nil
}

// GroupName returns the name for the gid.
func GroupName(gid int) (string, error) {
	groupsMu.RLock()
	isNil := groups == nil
	name := groups[gid]
	groupsMu.RUnlock()
	if isNil {
		groupsMu.Lock()
		if groups == nil {
			groups = make(map[int]string)
		}
		groupsMu.Unlock()
	}
	if name != "" {
		return name, nil
	}
	now := time.Now()
	if lastcheck.Add(1 * time.Second).After(now) { // fresh
		return getOrLookup(gid)
	}
	actcheck := lastcheck
	groupsMu.RUnlock()

	groupsMu.Lock()
	defer groupsMu.Unlock()
	if lastcheck != actcheck { // sy was faster
		return getOrLookup(gid)
	}
	fi, err := os.Stat(groupFile)
	if err != nil {
		return "", err
	}
	lastcheck = now
	if lastmod == fi.ModTime() { // no change
		return getOrLookup(gid)
	}

	// need to reread
	for k := range groups {
		delete(groups, k)
	}

	fh, err := os.Open(groupFile)
	if err != nil {
		return "", err
	}
	defer fh.Close()
	if fi, err = fh.Stat(); err != nil {
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

	return getOrLookup(gid)
}

// IsInsideDocker returns true iff we are inside a docker cgroup.
func IsInsideDocker() bool {
	_, err := ioutil.ReadFile("/proc/self/cgroup")
	if err == nil {
		return true
	}
	return false
}

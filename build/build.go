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

package build

import (
	"bytes"
	"fmt"
	"os/exec"
	"time"
)

// TagLDFlags appends gitTag, timestamp, commitHash ldflags to flags.
//
// To be used with "go build -ldflags".
func TagLDFlags(flags []string, dest string) []string {
	var tag string
	b, _ := exec.Command("git", "describe", "--tags").Output()
	if len(b) == 0 {
		tag = "dev"
	} else {
		tag = string(bytes.TrimSpace(b))
	}
	b, _ = exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	for k, v := range map[string]string{
		"gitTag":     tag,
		"timestamp":  time.Now().Format(time.RFC3339),
		"commitHash": string(bytes.TrimSpace(b)),
	} {
		flags = append(flags, fmt.Sprintf(`-X "%s.%s=%s"`, dest, k, v))
	}
	return flags
}

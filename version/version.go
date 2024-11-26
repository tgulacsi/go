// Copyright 2024 Tamás Gulácsi .All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"runtime/debug"
)

func Main() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		return ""
	}
	var vcsRev, vcsTime, vcsModified string
	for _, kv := range info.Settings {
		switch kv.Key {
		case "vcs.revision":
			vcsRev = kv.Value
		case "vcs.time":
			vcsTime = kv.Value
		case "vcs.modified":
			vcsModified = kv.Value
		}
	}
	if vcsModified == "false" {
		if info.Main.Version != "(devel)" || vcsRev == "" {
			return info.Main.Path + "@" + info.Main.Version
		}
		return info.Main.Path + "@" + vcsRev
	}
	return info.Main.Path + "@" + vcsRev + "-" + vcsTime
}

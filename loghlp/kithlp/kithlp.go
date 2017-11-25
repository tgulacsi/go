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

package kithlp

import (
	"fmt"

	"github.com/go-kit/kit/log/term"
)

var (
	// DefaultLevelColors is the default colors.
	DefaultLevelColors = map[string]term.FgBgColor{
		"crit":  {term.Magenta, term.DarkGray},
		"error": {term.Red, term.Default},
		"warn":  {term.Yellow, term.Default},
		"info":  {term.Default, term.Default},
		"debug": {term.Green, term.Default},
	}
)

// NewLevelColorer returns a function to be used in go-kit/kit/log/term.NewColorLogger.
// If levelColors is nil, the DefaultLevelColors is used.
func NewLevelColorer(
	levelName string,
	levelColors map[string]term.FgBgColor,
) func(keyvals ...interface{}) term.FgBgColor {
	if levelColors == nil {
		levelColors = DefaultLevelColors
	}
	return func(keyvals ...interface{}) term.FgBgColor {
		var level string

		for i := 0; i < len(keyvals); i += 2 {
			if keyvals[i] == levelName {
				level = asString(keyvals[i+1])
				break
			}
		}

		if level == "" {
			level = "info"
		}
		return levelColors[level]
	}
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	case fmt.Formatter:
		return fmt.Sprint(x)
	default:
		return fmt.Sprintf("%s", x)
	}
	return ""
}

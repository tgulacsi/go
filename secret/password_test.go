// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package secret_test

import (
	"testing"

	"github.com/go-json-experiment/json"
	"github.com/tgulacsi/go/secret"
)

func TestPassword(t *testing.T) {
	type Config struct {
		Username string
		Password secret.Password
	}
	conf := Config{
		Username: "user",
		Password: secret.Password("password"),
	}
	b, err := json.Marshal(conf)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("garbled:", string(b))
	if got, want := string(b), `{"Username":"user","Password":"p******d"}`; got != want {
		t.Errorf("got %s,\n wanted\n%s", got, want)
	}

	var conf2 Config
	if err := json.Unmarshal(b, &conf2); err != nil {
		t.Fatal(err)
	}
	if got := conf2.Password.String(); got != "" {
		t.Errorf("unmarshaled %q, wanted empty", got)
	}

	secret.MarshalPassword.Store(true)
	b, err = json.Marshal(conf)
	secret.MarshalPassword.Store(false)
	t.Log("real:", string(b))
	if err := json.Unmarshal(b, &conf2); err != nil {
		t.Fatal(err)
	}
	if conf != conf2 {
		t.Errorf("unmarshaled %#v, wanted %#v", conf2, conf)
	}
}

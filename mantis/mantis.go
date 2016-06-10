// Copyright 2014 Tamás Gulácsi. All rights reserved.

package mantis

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/kolo/xmlrpc"
	"github.com/pkg/errors"
	"github.com/tgulacsi/go/text"
	"gopkg.in/inconshreveable/log15.v2"
)

var Log = func(...interface{})error { return nil }

type Mantis struct {
	url         string
	user, passw string
}

func New(url, user, password string) Mantis {
	return Mantis{
		url:   url,
		user:  user,
		passw: password,
	}
}
func (m Mantis) Call(command string, args map[string]interface{}) (retval, error) {
	var ret retval
	req, err := xmlrpc.NewRequest(m.url, command, args)
	if err != nil {
		return ret, errors.Wrapf(err, "NewRequest(url=%q, method=%s, args=%+v)", m.url, command, args)
	}
	req.SetBasicAuth(m.user, m.passw)
	req.Header.Set("PHP_AUTH_USER", m.user)
	req.Header.Set("PHP_AUTH_PW", m.passw)

	//Log("msg","request", "body", log15.Lazy{func() (string, error) {
		b, err := httputil.DumpRequest(req, true)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}})

	cl := http.DefaultClient
	//Log("msg","Do", "req", req)
	resp, err := cl.Do(req)
	if err != nil {
		return ret, errors.Wrapf(err, "Do %#v", req)
	}
	defer resp.Body.Close()
	ret.Body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return ret, errors.Wrapf(err, "read resonse")
	}
	if resp.StatusCode >= 400 {
		return ret, errors.New(fmt.Sprintf("%q: %s: %s", req.URL, resp.Status, ret.Body))
	}
	Log("msg","response", "resp", resp)

	if ret.Body, err = ensureXmlUTF8(ret.Body); err != nil {
		Log("msg","transform encoding", "body", string(ret.Body), "error", err)
	}
	r := xmlrpc.NewResponse(ret.Body)
	if r.Failed() {
		return ret, r.Err()
	}
	if bytes.Contains(ret.Body, []byte("<name>errcode</name>")) {
		var ret retval
		if err = r.Unmarshal(&ret); err != nil {
			Log("msg","unmarshal retval", "body", string(ret.Body), "error", err)
		} else {
			if ret.Code == 0 {
				Log("msg","success")
				return ret, nil
			}
			Log("msg","failure", "errcode", ret.Code, "errmsg", ret.Msg)
			return ret, errors.New(fmt.Sprintf("failure; %d: %s", ret.Code, ret.Msg))
		}
	}
	Log("msg","success", "body", string(ret.Body))
	return ret, nil
}

func (m Mantis) NewUser(email, realName, userName string, accessLevel int) error {
	args := map[string]interface{}{
		"email":        email,
		"username":     userName,
		"realname":     url.QueryEscape(realName),
		"access_level": strconv.Itoa(accessLevel),
	}
	ret, err := m.Call("new_user", args)
	if err != nil {
		return err
	}
	if ret.Code == 0 {
		Log("msg","success")
		return nil
	}
	Log("msg","failure", "errcode", ret.Code, "errmsg", ret.Msg)
	return errors.New(fmt.Sprintf("failure; %d: %s", ret.Code, ret.Msg))
}

type retval struct {
	Code int    `xmlrpc:"errcode"`
	Msg  string `xmlrpc:"errmsg"`
	Body []byte
}

func (r retval) String() string {
	return fmt.Sprintf("%d: %s\n%s", r.Code, r.Msg, r.Body)
}

func ensureXmlUTF8(b []byte) ([]byte, error) {
	i := bytes.Index(b, []byte("?>"))
	if i <= 0 {
		return b, nil
	}
	//Log("msg","first", "i", i, "line", b[:i])
	j := bytes.Index(b[:i-1], []byte("encoding=\""))
	if j <= 0 {
		return b, nil
	}
	encS := string(bytes.ToLower(b[j+10 : i-1]))
	//Log("msg","enc", "j", j, "enc", encS)
	if encS == "utf-8" {
		return b, nil
	}
	enc := text.GetEncoding(encS)
	head, err := text.Decode(b[:j+10], enc)
	if err != nil {
		return b, fmt.Errorf("decode head: %v", err)
	}
	tail, err := text.Decode(b[i-1:], enc)
	if err != nil {
		return b, fmt.Errorf("decode tail: %v", err)
	}
	s := head + "utf-8" + tail
	//Log("msg","transformed", "body", s)
	return []byte(s), nil
}

// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: LGPL-3.0

package crypthlp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"software.sslmate.com/src/go-pkcs12"
)

// ReadPassword: see man openssl-passphrase-options
func ReadPassword(s string) (string, error) {
	var fh *os.File
	if s == "stdin" {
		fh = os.Stdin
	} else if typ, val, ok := strings.Cut(s, ":"); !ok {
		return s, nil
	} else {
		switch typ {
		case "pass":
			return val, nil
		case "env":
			return os.Getenv(val), nil
		case "file":
			var err error
			if fh, err = os.Open(val); err != nil {
				return "", err
			}
		case "fd":
			fd, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return s, fmt.Errorf("parse fd=%s as uint: %w", val, err)
			}
			fh = os.NewFile(uintptr(fd), "passin")
		}
	}
	if fh == nil {
		return s, nil
	}
	defer fh.Close()
	b, _, err := bufio.NewReader(fh).ReadLine()
	return string(b), err
}

// ParseP12Bytes parses the private key and certificate from the .p12 file using openssl.
func ParseP12Bytes(ctx context.Context, p12Bytes []byte, p12Password string) (privateKey any, certificate *x509.Certificate, err error) {
	if privateKey, certificate, err = pkcs12.Decode(p12Bytes, p12Password); err == nil {
		return
	}

	pk, cert, err := ReadP12Bytes(ctx, p12Bytes, p12Password)
	if err != nil {
		return nil, nil, err
	}
	if certificate, err = x509.ParseCertificate(cert); err != nil {
		return
	}
	for _, f := range []func([]byte) (any, error){
		func(b []byte) (any, error) { return x509.ParseECPrivateKey(b) },
		func(b []byte) (any, error) { return x509.ParsePKCS1PrivateKey(b) },
		x509.ParsePKCS8PrivateKey,
		x509.ParsePKIXPublicKey,
	} {
		if privateKey, err = f(pk); err == nil {
			return
		}
	}
	return
}

// ReadP12Bytes splits the give .p12 bytes to privateKey and certificate bytes (as returned by openssl).
func ReadP12Bytes(ctx context.Context, p12Bytes []byte, p12Password string) (privateKey, certificate []byte, err error) {
	if p12Password, err = ReadPassword(p12Password); err != nil {
		return nil, nil, err
	}

	// https://serverfault.com/questions/515833/how-to-remove-private-key-password-from-pkcs12-container
	// https://www.unosoft.hu/mantis/kobe/view.php?id=16930#c163679
	// How to remove the passphrase?
	noenc := "noenc"
	var out []byte
	if out, err = exec.CommandContext(ctx, "openssl", "pkcs12", "-h").CombinedOutput(); err != nil && len(out) == 0 {
		return
	} else if !bytes.Contains(out, []byte("-noenc")) { // older openssl
		noenc = "nodes"
	}

	const envName = "CRYPTO_PASSWORD"
	var buf bytes.Buffer
	var errBuf strings.Builder
	for _, argsDest := range []struct {
		Args []string
		Dest *[]byte
	}{
		{Args: []string{"-nokeys", "-clcerts"}, Dest: &certificate},
		// caPEM:   {"-nokeys", "-cacerts"},
		{Args: []string{"-nocerts", "-" + noenc}, Dest: &privateKey},
	} {
		buf.Reset()
		errBuf.Reset()
		cmd := exec.CommandContext(ctx, "openssl",
			append(append(make([]string, 0, 1+len(argsDest.Args)+2),
				"pkcs12", "-passin", "env:"+envName), argsDest.Args...)...)
		cmd.Stdin = bytes.NewReader(p12Bytes)
		cmd.Stdout = &buf
		cmd.Stderr = &errBuf
		cmd.Env = append(cmd.Env, envName+"="+p12Password)
		slog.Info("split", "cmd", cmd.Args)
		if err = cmd.Run(); err != nil {
			err = fmt.Errorf("%q: %s: %w", cmd.Args, errBuf.String(), err)
			return
		}
		if buf.Len() == 0 {
			slog.Warn("zero length", "cmd", cmd.Args)
		} else {
			*argsDest.Dest = append(make([]byte, 0, buf.Len()), buf.Bytes()...)
		}
	}
	return privateKey, certificate, nil
}

// ConvertP12 converts the .p12 file to <base64>.crt and <base64>.key PEM files. Does nothing if those files exist.
func ConvertP12(ctx context.Context, p12FileName, p12Password string) (certPEM, keyPEM string, err error) {
	b, readErr := os.ReadFile(p12FileName)
	if readErr != nil {
		err = readErr
		return
	}
	hsh := sha256.New224()
	hsh.Write(b)
	bn := filepath.Join(filepath.Dir(p12FileName), base64.RawURLEncoding.EncodeToString(hsh.Sum(nil)))
	certPEM, keyPEM = bn+".crt", bn+".key"
	var ok bool
	if fi, err := os.Stat(certPEM); err == nil && fi.Size() > 0 {
		fi, err = os.Stat(keyPEM)
		ok = err == nil && fi.Size() > 0
	}

	if ok {
		return
	}
	privateKey, certificate, err := ReadP12Bytes(ctx, b, p12Password)
	if err != nil {
		return "", "", err
	}
	if err = os.WriteFile(certPEM, certificate, 0600); err != nil {
		return "", "", err
	}
	return certPEM, keyPEM, os.WriteFile(keyPEM, privateKey, 0600)
}

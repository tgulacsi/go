// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: LGPL-3.0

package crypthlp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
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

type Bag struct {
	PrivateKey, Cert, CAs []byte
}

func ParseP12ToTLSCertificate(ctx context.Context, caCertPool *x509.CertPool, p12Bytes []byte, p12Password string) (tls.Certificate, error) {
	bag, err := ReadP12Bytes(ctx, p12Bytes, p12Password, false)
	if err != nil {
		return tls.Certificate{}, err
	}
	return bag.ToTLSCertificate(caCertPool)
}

func (bag Bag) ToTLSCertificate(caCertPool *x509.CertPool) (tls.Certificate, error) {
	priv, crt, cas, err := bag.Parse()
	if err != nil {
		return tls.Certificate{}, err
	}
	cert := tls.Certificate{
		Certificate: [][]byte{crt.Raw},
		Leaf:        crt,
	}
	for _, ca := range cas {
		caCertPool.AddCert(ca)
	}
	var ok bool
	switch crt.PublicKey.(type) {
	case *rsa.PublicKey:
		if cert.PrivateKey, ok = priv.(*rsa.PrivateKey); !ok {
			return cert, fmt.Errorf("tls: private key type does not match public key type")
		}
	case *ecdsa.PublicKey:
		if cert.PrivateKey, ok = priv.(*ecdsa.PrivateKey); !ok {
			return cert, fmt.Errorf("tls: private key type does not match public key type")
		}
	case ed25519.PublicKey:
		if cert.PrivateKey, ok = priv.(ed25519.PrivateKey); !ok {
			return cert, fmt.Errorf("tls: private key type does not match public key type")
		}
	default:
		return cert, fmt.Errorf("tls: unknown public key algorithm")
	}
	return cert, nil
}

// ParseP12Bytes parses the private key and certificate from the .p12 file using openssl.
func ParseP12Bytes(ctx context.Context, p12Bytes []byte, p12Password string) (privateKey any, certificate *x509.Certificate, CAs []*x509.Certificate, err error) {
	if privateKey, certificate, err = pkcs12.Decode(p12Bytes, p12Password); err == nil {
		return
	}

	bag, err := ReadP12Bytes(ctx, p12Bytes, p12Password, false)
	if err != nil {
		return nil, nil, nil, err
	}
	return bag.Parse()
}

// Parse parses the private key and certificate from the bytes
func (bag Bag) Parse() (privateKey any, certificate *x509.Certificate, CAs []*x509.Certificate, err error) {
	if certificate, err = x509.ParseCertificate(bag.Cert); err != nil {
		return
	}

	if len(bag.CAs) != 0 {
		if CAs, err = x509.ParseCertificates(bag.CAs); err != nil {
			slog.Error("parse cas", "error", err)
		}
	}
	for _, f := range []func([]byte) (any, error){
		func(b []byte) (any, error) { return x509.ParseECPrivateKey(b) },
		func(b []byte) (any, error) { return x509.ParsePKCS1PrivateKey(b) },
		x509.ParsePKCS8PrivateKey,
		x509.ParsePKIXPublicKey,
	} {
		if privateKey, err = f(bag.PrivateKey); err == nil {
			return
		}
	}
	return
}

// ReadP12Bytes splits the give .p12 bytes to privateKey and certificate bytes (as returned by openssl).
func ReadP12Bytes(ctx context.Context, p12Bytes []byte, p12Password string, outputPEM bool) (Bag, error) {
	p12Password, err := ReadPassword(p12Password)
	if err != nil {
		return Bag{}, err
	}

	// https://serverfault.com/questions/515833/how-to-remove-private-key-password-from-pkcs12-container
	// https://www.unosoft.hu/mantis/kobe/view.php?id=16930#c163679
	// How to remove the passphrase?
	noenc := "noenc"
	var bag Bag
	var out []byte
	if out, err = exec.CommandContext(ctx, "openssl", "pkcs12", "-h").CombinedOutput(); err != nil && len(out) == 0 {
		return bag, err
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
		{Args: []string{"-nokeys", "-clcerts"}, Dest: &bag.Cert},
		{Args: []string{"-nokeys", "-cacerts"}, Dest: &bag.CAs},
		{Args: []string{"-nocerts", "-" + noenc}, Dest: &bag.PrivateKey},
	} {
		buf.Reset()
		errBuf.Reset()
		cmd := exec.CommandContext(ctx, "openssl",
			append(append(make([]string, 0, 1+len(argsDest.Args)+4),
				"pkcs12", "-passin", "env:"+envName),
				argsDest.Args...)...)
		cmd.Stdin = bytes.NewReader(p12Bytes)
		cmd.Stdout = &buf
		cmd.Stderr = &errBuf
		cmd.Env = append(cmd.Env, envName+"="+p12Password)
		slog.Debug("split", "cmd", cmd.Args)
		if err = cmd.Run(); err != nil {
			return bag, fmt.Errorf("%q: %s: %w", cmd.Args, errBuf.String(), err)
		}
		if buf.Len() == 0 {
			slog.Warn("zero length", "cmd", cmd.Args)
		} else {
			if slog.Default().Enabled(ctx, slog.LevelDebug) {
				slog.Debug("got", "bytes", buf.String())
			}
			if outputPEM {
				*argsDest.Dest = append(make([]byte, 0, buf.Len()), buf.Bytes()...)
			} else {
				if p, _ := pem.Decode(buf.Bytes()); p != nil {
					*argsDest.Dest = append(make([]byte, 0, len(p.Bytes)), p.Bytes...)
				}
			}
		}
	}
	return bag, nil
}

// ConvertP12 converts the .p12 file to <base64>.crt and <base64>.key PEM files. Does nothing if those files exist.
func ConvertP12(ctx context.Context, p12FileName, p12Password string) (certPEM, caPEM, keyPEM string, err error) {
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
	bag, err := ReadP12Bytes(ctx, b, p12Password, true)
	if err != nil {
		return "", "", "", err
	}
	if err = os.WriteFile(certPEM, bag.Cert, 0600); err != nil {
		return "", "", "", err
	}
	if err = os.WriteFile(caPEM, bag.CAs, 0600); err != nil {
		return certPEM, "", "", err
	}
	return certPEM, caPEM, keyPEM, os.WriteFile(keyPEM, bag.PrivateKey, 0600)
}

// ReadJKSBytes splits the give .p12 bytes to privateKey and certificate bytes (as returned by openssl).
func ReadJKSBytes(ctx context.Context, jksBytes []byte, jksPassword string, outputPEM bool) (Bag, error) {
	fh, err := os.CreateTemp("", "jks-*.jks")
	if err != nil {
		return Bag{}, err
	}
	defer func() { fh.Close(); os.Remove(fh.Name()) }()
	if _, err = fh.Write(jksBytes); err != nil {
		return Bag{}, err
	}
	if fh.Close(); err != nil {
		return Bag{}, err
	}
	p12Fn := fh.Name() + ".p12"
	if b, err := exec.CommandContext(ctx, "keytool",
		"-importkeystore",
		"-srckeystore", fh.Name(),
		"-destkeystore", p12Fn, "-deststoretype", "pkcs12",
		"-srcstorepass", jksPassword, "-deststorepass", jksPassword,
	).CombinedOutput(); err != nil {
		slog.Warn("convert JKS to P12", "out", string(b), "error", err)
	} else {
		defer os.Remove(p12Fn)
		p12Bytes, err := os.ReadFile(p12Fn)
		if err != nil {
			return Bag{}, err
		}
		return ReadP12Bytes(ctx, p12Bytes, jksPassword, outputPEM)
	}

	// Plan B
	p12Bytes, err := exec.CommandContext(ctx, "keytool",
		"-list", "-keystore", fh.Name(),
		"-storepass", jksPassword, "-storetype", "JKS",
		"-rfc",
	).Output()
	if err != nil {
		return Bag{}, err
	}
	return ReadP12Bytes(ctx, p12Bytes, jksPassword, outputPEM)
}

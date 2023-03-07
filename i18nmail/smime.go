// Copyright 2023 Tamas Gulacsi. All rights reserved.

package i18nmail

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/mail"
	"os/exec"
	"strings"
)

// DecodeSMIME decodes S/MIME smime.p7m if that's the only part.
func DecodeSMIME(ctx context.Context, sr *io.SectionReader) (*io.SectionReader, error) {
	msg, err := mail.ReadMessage(io.NewSectionReader(sr, 0, sr.Size()))
	if err != nil {
		return sr, err
	}
	ct := msg.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/x-pkcs7-mime") {
		return sr, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx,
		"openssl", "smime", "-verify",
		"-noverify", "-nosigs",
		"-in", "-")
	cmd.Stdin = io.NewSectionReader(sr, 0, sr.Size())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err = cmd.Run(); err != nil {
		return sr, fmt.Errorf("%v: %w\n%s", cmd.Args, err, stderr.String())
	}
	return io.NewSectionReader(bytes.NewReader(stdout.Bytes()), 0, int64(stdout.Len())), nil
}

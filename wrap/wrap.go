// Package wrap handles simple text wrapping to a fixed number of columns.
package wrap

import (
	"bufio"
	"bytes"
	"io"
)

// Wrap a string to the specified number of columns along word breaks (space
// character). Output will use \n (newline) characters only. No newlines or
// carriage returns in the input will be preserved, which means that if you
// pass in text containing several paragraphs you'll get one giant paragraph
// back. If you wish to avoid that, then split your text into paragraphs and
// call wrap.String individually on each paragraph.
func String(s string, numCols uint) string {
	var (
		ob  bytes.Buffer // "output buffer" (for storing the finished output)
		lb  bytes.Buffer // "lookahead buffer" (a temporary buffer for storing intermediate characters read while we're looking ahead for the next space)
		col uint         // number of chars written to output buffer since last newline
	)

	// Check for a nonsense number of columns and simply return the input
	// string. This avoids us needing to have an error return for this case.
	if numCols == 0 {
		return s
	}

	isNewline := true // true if we haven't written any chars to this line of the output buffer yet

	for i, v := range s {
		switch v {
		case ' ', '\n', '\r':
			// is the temp buffer empty? If so, ignore this because it's repeated
			// whitespace.
			if lb.Len() == 0 {
				continue
			}

			// write a space before copying the lookahead buffer in the case that
			// this was not the first time we flushed the lookahead buffer on this
			// output line.
			if !isNewline {
				ob.WriteRune(' ')
				col += 1
			}

			col += uint(lb.Len())

			// write all the characters we're stored in the temp buffer (excluding
			// this space, in case this is the last one we'll write for this line)
			// into the output buffer.
			io.Copy(&ob, &lb)
			isNewline = false
		default:
			// Not a space or a newline, so just add this character to the
			// temporary buffer.
			lb.WriteRune(v)
		}
		if col+uint(lb.Len()) >= numCols {
			if isNewline {
				// If isNewline is true, then we've hit the maximum number of
				// columns allowed without having found a single space. So flush
				// the lookahead buffer to the output buffer.
				io.Copy(&ob, &lb)
			}

			// If isNewline is false, then we already wrote some output to this
			// line. So, preserve the temporary buffer's contents.

			// Only write a newline if we aren't on exactly the last character of
			// input or the temp buffer isn't empty (helps prevent
			// possibly-unwanted trailing newlines).
			if i != len(s)-1 || lb.Len() != 0 {
				ob.WriteRune('\n')
			}

			// Ok, now reset our state for a new line.
			col = 0
			isNewline = true
		}
	}

	// All done with input! Dump the temporary buffer to output (possibly adding
	// a space beforehand)
	if !isNewline {
		ob.WriteRune(' ')
	}
	io.Copy(&ob, &lb)

	return ob.String()
}

// WrappingReader returns an io.Reader which will wrap lines longer than the given width.
// All other lines (LF chars) will be preserved.
func WrappingReader(r io.Reader, width uint) io.Reader {
	pr, pw := io.Pipe()

	go func() {
		scanner := bufio.NewScanner(r)
		ew := &errWriter{Writer: pw}
		for scanner.Scan() { // split lines
			if uint(len(scanner.Bytes())) <= width {
				ew.Write(scanner.Bytes())
				if _, err := ew.Write([]byte{'\n'}); err != nil {
					break
				}
				continue
			}
			io.WriteString(ew, String(scanner.Text(), width))
			if _, err := ew.Write([]byte{'\n'}); err != nil {
				break
			}
		}
		err := scanner.Err()
		if err == nil {
			err = ew.Err()
		}
		pw.CloseWithError(err)
	}()

	return pr
}

type errWriter struct {
	io.Writer
	err error
}

func (w *errWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	var n int
	n, w.err = w.Writer.Write(p)
	return n, w.err
}
func (w *errWriter) Err() error { return w.err }

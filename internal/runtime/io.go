package runtime

import (
	"bytes"
	"io"
	"sync"
)

// maxCapture bounds how much stdout/stderr is retained per task in state.
const maxCapture = 1 << 20 // 1 MiB

// capBuf is a size-capped buffer used to retain command output for state.
type capBuf struct {
	buf bytes.Buffer
}

func (c *capBuf) Write(p []byte) (int, error) {
	if c.buf.Len() >= maxCapture {
		return len(p), nil
	}
	room := maxCapture - c.buf.Len()
	if len(p) > room {
		c.buf.Write(p[:room])
		return len(p), nil
	}
	return c.buf.Write(p)
}

func (c *capBuf) String() string { return c.buf.String() }

// linePrefixWriter prefixes each complete line with "[name] " before writing to
// the underlying stream. It is safe for concurrent tasks sharing one stream.
type linePrefixWriter struct {
	mu     *sync.Mutex
	w      io.Writer
	prefix []byte
	pend   []byte
}

var streamMu sync.Mutex

func prefixWriter(w io.Writer, name string) io.Writer {
	return &linePrefixWriter{mu: &streamMu, w: w, prefix: []byte("[" + name + "] ")}
}

func (l *linePrefixWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pend = append(l.pend, p...)
	for {
		i := bytes.IndexByte(l.pend, '\n')
		if i < 0 {
			break
		}
		line := l.pend[:i+1]
		if _, err := l.w.Write(l.prefix); err != nil {
			return len(p), err
		}
		if _, err := l.w.Write(line); err != nil {
			return len(p), err
		}
		l.pend = l.pend[i+1:]
	}
	return len(p), nil
}

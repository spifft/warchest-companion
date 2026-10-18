package chatlog

import (
	"bytes"
	"io"
	"os"
)

// Tailer follows the chat log like `tail -f`: it starts at the end (old chat is never sent),
// returns only complete new lines, and starts over when the game makes a new log file.
type Tailer struct {
	Path    string
	offset  int64
	started bool
	partial []byte
}

// Poll returns the lines written since the last call.
func (t *Tailer) Poll() ([]string, error) {
	f, err := os.Open(t.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	if !t.started {
		t.offset, t.started = size, true // begin at the end
		return nil, nil
	}
	if size < t.offset { // a new, shorter file: read it from the start
		t.offset, t.partial = 0, nil
	}
	if size == t.offset {
		return nil, nil
	}
	// at most 1 MB at a time (a busy evening is far less)
	n := size - t.offset
	if n > 1<<20 {
		t.offset = size - 1<<20
		n = 1 << 20
		t.partial = nil
	}
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, t.offset); err != nil && err != io.EOF {
		return nil, err
	}
	t.offset += n
	data := append(t.partial, buf...)
	last := bytes.LastIndexByte(data, '\n')
	if last < 0 {
		t.partial = data
		return nil, nil
	}
	t.partial = append([]byte(nil), data[last+1:]...)
	var out []string
	for _, l := range bytes.Split(data[:last], []byte{'\n'}) {
		if len(l) > 0 {
			out = append(out, string(bytes.TrimRight(l, "\r")))
		}
	}
	return out, nil
}

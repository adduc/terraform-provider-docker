package internal

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"

	"github.com/moby/moby/api/pkg/stdcopy"
)

// frame encodes msg as one multiplexed log frame: a stream byte, three zero
// bytes, the big-endian payload length, then the payload.
func frame(stream stdcopy.StdType, msg string) []byte {
	hdr := make([]byte, 8)
	hdr[0] = byte(stream)
	binary.BigEndian.PutUint32(hdr[4:], uint32(len(msg)))
	return append(hdr, msg...)
}

func frames(fs ...[]byte) *bytes.Reader {
	return bytes.NewReader(bytes.Join(fs, nil))
}

const testTimestamp = "2026-10-03T12:34:56.123456789Z"

func TestReadLogs(t *testing.T) {
	long := strings.Repeat("x", 100000)

	tests := []struct {
		name       string
		input      *bytes.Reader
		tty        bool
		timestamps bool
		want       []logEntry
	}{
		{
			name:  "empty",
			input: frames(),
			want:  nil,
		},
		{
			// A 10-byte payload has 0x0A (newline) as its last header byte.
			name:  "length header containing newline",
			input: frames(frame(stdcopy.Stdout, "012345678\n")),
			want:  []logEntry{{Stream: stdcopy.Stdout, Message: "012345678"}},
		},
		{
			name: "stdout and stderr",
			input: frames(
				frame(stdcopy.Stdout, "out\n"),
				frame(stdcopy.Stderr, "err\n"),
			),
			want: []logEntry{
				{Stream: stdcopy.Stdout, Message: "out"},
				{Stream: stdcopy.Stderr, Message: "err"},
			},
		},
		{
			name:       "timestamps",
			input:      frames(frame(stdcopy.Stdout, testTimestamp+" hello world\n")),
			timestamps: true,
			want:       []logEntry{{Stream: stdcopy.Stdout, Timestamp: testTimestamp, Message: "hello world"}},
		},
		{
			name:       "empty line with timestamp",
			input:      frames(frame(stdcopy.Stdout, testTimestamp+" \n")),
			timestamps: true,
			want:       []logEntry{{Stream: stdcopy.Stdout, Timestamp: testTimestamp, Message: ""}},
		},
		{
			name:  "line longer than scanner buffer",
			input: frames(frame(stdcopy.Stdout, long+"\n")),
			want:  []logEntry{{Stream: stdcopy.Stdout, Message: long}},
		},
		{
			// The daemon splits long lines into partial messages, each with
			// its own timestamp; stderr output in between must not join them.
			name: "partial messages are joined",
			input: frames(
				frame(stdcopy.Stdout, testTimestamp+" first "),
				frame(stdcopy.Stderr, "2026-10-03T12:34:57.000000000Z err\n"),
				frame(stdcopy.Stdout, "2026-10-03T12:34:58.000000000Z half\n"),
			),
			timestamps: true,
			want: []logEntry{
				{Stream: stdcopy.Stdout, Timestamp: testTimestamp, Message: "first half"},
				{Stream: stdcopy.Stderr, Timestamp: "2026-10-03T12:34:57.000000000Z", Message: "err"},
			},
		},
		{
			name:  "unterminated final line",
			input: frames(frame(stdcopy.Stdout, "a\nb")),
			want: []logEntry{
				{Stream: stdcopy.Stdout, Message: "a"},
				{Stream: stdcopy.Stdout, Message: "b"},
			},
		},
		{
			name:  "tty",
			input: bytes.NewReader([]byte("hello\r\nworld\r\n")),
			tty:   true,
			want: []logEntry{
				{Stream: stdcopy.Stdout, Message: "hello"},
				{Stream: stdcopy.Stdout, Message: "world"},
			},
		},
		{
			name:       "tty with timestamps",
			input:      bytes.NewReader([]byte(testTimestamp + " hello\r\n")),
			tty:        true,
			timestamps: true,
			want:       []logEntry{{Stream: stdcopy.Stdout, Timestamp: testTimestamp, Message: "hello"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readLogs(tt.input, tt.tty, tt.timestamps)
			if err != nil {
				t.Fatalf("readLogs: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readLogs:\n got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestReadLogsErrors(t *testing.T) {
	tests := []struct {
		name       string
		input      *bytes.Reader
		timestamps bool
		want       string
	}{
		{
			name:       "missing timestamp",
			input:      frames(frame(stdcopy.Stdout, "no-timestamp")),
			timestamps: true,
			want:       "no timestamp",
		},
		{
			// Stream type 3 carries an error message from the daemon.
			name:  "daemon error",
			input: frames(frame(stdcopy.Systemerr, "something broke")),
			want:  "something broke",
		},
		{
			name:  "unknown stream type",
			input: frames(frame(9, "x")),
			want:  "unrecognized stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readLogs(tt.input, false, tt.timestamps)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("readLogs error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

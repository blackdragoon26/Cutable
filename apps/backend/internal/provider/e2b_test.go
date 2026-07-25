package provider

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"testing"
)

func TestDecodeCommandStream(t *testing.T) {
	stdout := base64.StdEncoding.EncodeToString([]byte("hello\n"))
	stderr := base64.StdEncoding.EncodeToString([]byte("warning\n"))
	events := [][]byte{
		[]byte(fmt.Sprintf(`{"event":{"data":{"stdout":%q}}}`, stdout)),
		[]byte(fmt.Sprintf(`{"event":{"data":{"stderr":%q}}}`, stderr)),
		[]byte(`{"event":{"end":{"exitCode":2}}}`),
	}
	var stream bytes.Buffer
	for _, event := range events {
		stream.WriteByte(0)
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(event)))
		stream.Write(length[:])
		stream.Write(event)
	}
	result, err := decodeCommandStream(&stream)
	if err != nil {
		t.Fatalf("decodeCommandStream() error = %v", err)
	}
	if result.Stdout != "hello\n" || result.Stderr != "warning\n" || result.ExitCode != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

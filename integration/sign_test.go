package main

import (
	"errors"
	"github.com/go-check/check"
	"io"
	"strings"
)

const (
	gpgBinary = "gpg"
)

func init() {
	check.Suite(&SigningSuite{})
}

type SigningSuite struct {
	gpgHome     string
	fingerprint string
}

func findFingerprint(lineBytes []byte) (string, error) {
	lines := string(lineBytes)
	for _, line := range strings.Split(lines, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) >= 10 && fields[0] == "fpr" {
			return fields[9], nil
		}
	}
	return "", errors.New("No fingerprint found")
}

func ConsumeAndLogOutput(c *check.C, id string, f io.ReadCloser, err error) {
	c.Assert(err, check.IsNil)
	go func() {
		defer func() {
			f.Close()
			c.Logf("Output %s: Closed", id)
		}()
		buf := make([]byte, 0, 1024)
		for {
			c.Logf("Output %s: waiting", id)
			n, err := f.Read(buf)
			c.Logf("Output %s: got %d,%#v: %#v", id, n, err, buf[:n])
			if n <= 0 {
				break
			}
		}
	}()
}

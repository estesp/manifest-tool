package main

import (
	"github.com/go-check/check"
	"io"
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

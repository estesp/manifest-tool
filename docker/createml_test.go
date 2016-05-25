package docker

import (
        "testing"
"github.com/docker/distribution/registry/api/v2"
        "github.com/docker/docker/reference"
)
func TeststatusSuccess(t *testing.T) {
        var crctstatus = []struct {
                a int
        }{
                {200},
                {239},
                {278},
                {300},
                {399},
          }
        var wrngstatus = []struct {
                b int
        }{
                {1},
                {50},
                {111},
                {199},
                {400},
                {1000},
        }

        for _, i := range crctstatus {
                res := statusSuccess(i.a)
                if res != true {
                        t.Errorf("%d is an invalid status", i.a)
                }

                for _, j := range wrngstatus {
                        res := statusSuccess(j.b)
                        if res == true {
                                t.Errorf("%d is an invalid status", j.b)
                        }

                }
        }
}

func  (t *testing.T) {
        urlBuilder, err := v2.NewURLBuilderFromString("https://myregistrydomain.com:5000")
        name := "debian:latest"
        ref, _ := reference.ParseNamed(name)
        url, err := createManifestURLFromRef(ref, urlBuilder)
        if url != " https://myregistrydomain.com:5000/v2/debian/manifests/latest" {
                t.Errorf("Error setting up repository endpoint and references for %q: %v", ref, err)
        }
}

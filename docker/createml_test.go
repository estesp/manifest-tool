package docker

import (
       "strings"
        "testing"
        "github.com/docker/distribution/registry/api/v2"
        "github.com/docker/docker/reference"
        "github.com/docker/docker/registry"
        "github.com/estesp/manifest-tool/vendor/github.com/docker/docker/registry"
        registrytypes "github.com/docker/engine-api/types/registry"
        "github.com/estesp/manifest-tool/vendor/golang.org/x/net/context"
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

func  TestcreateManifestURLFromRef(t *testing.T) {
        urlBuilder, err := v2.NewURLBuilderFromString("https://myregistrydomain.com:5000")
        name := "debian:latest"
        ref, _ := reference.ParseNamed(name)
        url, err := createManifestURLFromRef(ref, urlBuilder)
        if url != " https://myregistrydomain.com:5000/v2/debian/manifests/latest" {
                t.Errorf("Error setting up repository endpoint and references for %q: %v", ref, err)
        }
}

func TestsetupRepo(t *testing.T) {
        var err fallbackError
        var res string
        name := "docker.io/debian"
        ref,_ := reference.ParseNamed(name)
        repoinf := registry.RepositoryInfo{
                ref,
                Index: *registrytypes.IndexInfo{
                        Name:  "myregistrydomain.com:5000",
                        Mirrors: {},
                        Secure:  false,
                        Official:  false,
                },
                Official: false,
        }
        _,res,err == setupRepo(repoinf)
                if err != nil{
                t.Errorf("Error setting up repository reponame %s", res)
                }
        }

func TestPutManifestList(t *testing.T) {
        ctx :=context.Background()
        filePath := "/home/mathew/listm.yml"
        str,err := PutManifestList(ctx, filePath)
        if err != nil{
                t.Errorf("Error in PutManifestList %s", str)
        }
}
func TestgetHTTPClient(t*testing.T){
        c := context.Background()
        repoinf := registry.RepositoryInfo{
                ref,
                Index: *registrytypes.IndexInfo{
                        Name:  "myregistrydomain.com:5000",
                        Mirrors: {},
                        Secure:  false,
                        Official:  false,
                },
                Official: false,
        }
        reponame := "hello-world"
        cli,err := getHTTPClient(c,repoinf,reponame)
        if err != nil{
                t.Errorf("Error in  getHTTPClient%s", str)
        }
}
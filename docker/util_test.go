package docker

import "testing"

func TestValidOSArch(t *testing.T) {
	var crctosarch = []struct {
		arch, os, variant string
	}{
		{arch: "darwin", os: "386"},
		{arch: "linux", os: "arm"},
		{arch: "windows", os: "amd64"},
		{arch: "darwin", os: "386"},
		{arch: "darwin", os: "amd64"},
		{arch: "darwin", os: "arm"},
		{arch: "darwin", os: "arm64"},
		{arch: "dragonfly", os: "amd64"},
		{arch: "freebsd", os: "386"},
		{arch: "freebsd", os: "amd64"},
		{arch: "freebsd", os: "arm"},
		{arch: "linux", os: "386"},
		{arch: "linux", os: "amd64"},
		{arch: "linux", os: "arm"},
		{arch: "linux", os: "arm", variant: "v5"},
		{arch: "linux", os: "arm", variant: "v6"},
		{arch: "linux", os: "arm", variant: "v7"},
		{arch: "linux", os: "arm64"},
		{arch: "linux", os: "arm64", variant: "v8"},
		{arch: "linux", os: "ppc64"},
		{arch: "linux", os: "ppc64le"},
		{arch: "linux", os: "mips64"},
		{arch: "linux", os: "mips64le"},
		{arch: "linux", os: "s390x"},
		{arch: "netbsd", os: "386"},
		{arch: "netbsd", os: "amd64"},
		{arch: "netbsd", os: "arm"},
		{arch: "openbsd", os: "386"},
		{arch: "openbsd", os: "amd64"},
		{arch: "openbsd", os: "arm"},
		{arch: "plan9", os: "386"},
		{arch: "plan9", os: "amd64"},
		{arch: "solaris", os: "amd64"},
		{arch: "windows", os: "386"},
		{arch: "windows", os: "amd64"},
	}
	var wrongosarch = []struct {
		arch, os, variant string
	}{
		{arch: "abc", os: "123"},
		{arch: "xyz", os: "etc"},
		{arch: "", os: ""},
	}

	for _, i := range crctosarch {
		res := isValidOSArch(i.arch, i.os, i.variant)
		if res != true {
			t.Errorf("%s/%s/%s is an invalid os/arch or os/arch/variant combination", i.arch, i.os, i.variant)
		}
	}

	for _, j := range wrongosarch {
		res := isValidOSArch(j.arch, j.os, j.variant)
		if res == true {
			t.Errorf("%s/%s/%s is an invalid os/arch or os/arch/variant combination", j.arch, j.os, j.variant)
		}
	}

}

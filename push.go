package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/estesp/manifest-tool/docker"
	"github.com/estesp/manifest-tool/types"
	"github.com/go-yaml/yaml"
	"github.com/sirupsen/logrus"

	"github.com/docker/distribution/manifest/manifestlist"
)

var pushCmd = cli.Command{
	Name:  "push",
	Usage: "push a manifest list entry to a registry with provided image details",
	Subcommands: []cli.Command{
		{
			Name:  "from-spec",
			Usage: "push a manifest list to a registry via a YAML spec",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "ignore-missing",
					Usage: "only warn on missing images defined in YAML spec",
				},
			},
			Action: func(c *cli.Context) {

				filePath := c.Args().First()
				a := getAuthInfo(c)
				ignoreMissing := c.Bool("ignore-missing")
				var yamlInput types.YAMLInput

				filename, err := filepath.Abs(filePath)
				if err != nil {
					logrus.Fatalf(fmt.Sprintf("Can't resolve path to %q: %v", filePath, err))
				}
				yamlFile, err := ioutil.ReadFile(filename)
				if err != nil {
					logrus.Fatalf(fmt.Sprintf("Can't read YAML file %q: %v", filePath, err))
				}
				err = yaml.Unmarshal(yamlFile, &yamlInput)
				if err != nil {
					logrus.Fatalf(fmt.Sprintf("Can't unmarshal YAML file %q: %v", filePath, err))
				}

				digest, l, err := docker.PutManifestList(a, yamlInput, ignoreMissing)
				if err != nil {
					logrus.Fatal(err)
				}
				fmt.Printf("Digest: %s %d\n", digest, l)
			},
		},
		{
			Name:  "from-args",
			Usage: "push a manifest list to a registry via CLI arguments",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "platforms",
					Usage: "comma-separated list of the platforms that images should be pushed for",
				},
				cli.StringFlag{
					Name:  "template",
					Usage: "the pattern the source images have. OS and ARCH in that pattern will be replaced with the actual values from the platforms list",
				},
				cli.StringFlag{
					Name:  "target",
					Usage: "the name of the manifest list image that is going to be produced",
				},
				cli.BoolFlag{
					Name:  "ignore-missing",
					Usage: "only warn on missing images defined in platform list",
				},
			},
			Action: func(c *cli.Context) {

				a := getAuthInfo(c)
				platforms := c.String("platforms")
				templ := c.String("template")
				target := c.String("target")
				ignoreMissing := c.Bool("ignore-missing")
				srcImages := []types.ManifestEntry{}

				if len(platforms) == 0 || len(templ) == 0 || len(target) == 0 {
					logrus.Fatalf("You must specify all three arguments --platforms, --template and --target")
				}

				platformList := strings.Split(platforms, ",")

				for _, platform := range platformList {
					osArchArr := strings.Split(platform, "/")
					if len(osArchArr) < 2 || len(osArchArr) > 3 {
						logrus.Fatal("The --platforms argument must be a string slice where one value is of the form 'os/arch/[variant]'")
					}
					os, arch := osArchArr[0], osArchArr[1]
					variant := ""
					if len(osArchArr) == 3 {
						variant = osArchArr[2]
					}
					srcImages = append(srcImages, types.ManifestEntry{
						Image: strings.Replace(strings.Replace(strings.Replace(templ, "ARCH", arch, 1), "OS", os, 1), "VARIANT", variant, 1),
						Platform: manifestlist.PlatformSpec{
							OS:           os,
							Architecture: arch,
							Variant:      variant,
						},
					})
				}

				yamlInput := types.YAMLInput{
					Image:     target,
					Manifests: srcImages,
				}

				digest, l, err := docker.PutManifestList(a, yamlInput, ignoreMissing)
				if err != nil {
					logrus.Fatal(err)
				}
				fmt.Printf("Digest: %s %d\n", digest, l)
			},
		},
	},
}

func getAuthInfo(c *cli.Context) *types.AuthInfo {
	return &types.AuthInfo{
		Username:  c.GlobalString("username"),
		Password:  c.GlobalString("password"),
		DockerCfg: c.GlobalString("docker-cfg"),
	}
}

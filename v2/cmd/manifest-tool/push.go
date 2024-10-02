package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/estesp/manifest-tool/v2/pkg/registry"
	"github.com/estesp/manifest-tool/v2/pkg/types"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	yaml "gopkg.in/yaml.v3"
)

const (
	fmtCantResolvePath   = "Can't resolve path to %q: %v"
	fmtCantReadYAML      = "Can't read YAML file %q: %v"
	fmtCantUnmarshalYAML = "Can't unmarshal YAML file %q: %v"
)

var pushCmd = &cli.Command{
	Name:  "push",
	Usage: "push a manifest list/OCI index entry to a registry with provided image details",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "type",
			Value: "docker",
			Usage: "image manifest type: docker (v2.2 manifest list) or oci (v1 index)",
		},
	},
	Subcommands: []*cli.Command{
		{
			Name:  "from-spec",
			Usage: "push a manifest list to a registry via a YAML spec",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "ignore-missing",
					Usage: "only warn on missing images defined in YAML spec",
				},
			},
			Action: func(c *cli.Context) error {
				filePath := c.Args().First()
				var yamlInput types.YAMLInput

				filename, err := filepath.Abs(filePath)
				if err != nil {
					logrus.Fatal(fmt.Sprintf(fmtCantResolvePath, filePath, err))
				}
				yamlFile, err := os.ReadFile(filename)
				if err != nil {
					logrus.Fatal(fmt.Sprintf(fmtCantReadYAML, filePath, err))
				}
				err = yaml.Unmarshal(yamlFile, &yamlInput)
				if err != nil {
					logrus.Fatal(fmt.Sprintf(fmtCantUnmarshalYAML, filePath, err))
				}

				manifestType := types.Docker
				if c.String("type") == "oci" {
					manifestType = types.OCI
				}
				digest, length, err := registry.PushManifestList(c.String("username"), c.String("password"), yamlInput, c.Bool("ignore-missing"), c.Bool("insecure"), c.Bool("plain-http"), manifestType, c.String("docker-cfg"))
				if err != nil {
					logrus.Fatal(err)
				}
				fmt.Printf("Digest: %s %d\n", digest, length)

				return nil
			},
		},
		{
			Name:  "from-args",
			Usage: "push a manifest list to a registry via CLI arguments",
			Flags: []cli.Flag{
				&cli.StringSliceFlag{
					Name:     "platforms",
					Usage:    "comma-separated list of the platforms that images should be pushed for",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "template",
					Usage:    "the pattern the source images have. OS and ARCH in that pattern will be replaced with the actual values from the platforms list",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "target",
					Usage:    "the name of the manifest list image that is going to be produced",
					Required: true,
				},
				&cli.StringSliceFlag{
					Name:  "tags",
					Usage: "comma-separated list of additional tags to apply to the manifest list image",
				},
				&cli.BoolFlag{
					Name:  "ignore-missing",
					Usage: "only warn on missing images defined in platform list",
				},
			},
			Action: func(c *cli.Context) error {
				platforms := c.StringSlice("platforms")
				templ := c.String("template")
				target := c.String("target")
				tags := c.StringSlice("tags")
				srcImages := []types.ManifestEntry{}

				for _, platform := range platforms {
					osArchArr := strings.Split(platform, "/")
					if len(osArchArr) != 2 && len(osArchArr) != 3 {
						logrus.Fatal("The --platforms argument must be a string slice where one value is of the form 'os/arch'")
					}
					variant := ""
					os, arch := osArchArr[0], osArchArr[1]
					if len(osArchArr) == 3 {
						variant = osArchArr[2]
					}
					srcImages = append(srcImages, types.ManifestEntry{
						Image: strings.Replace(strings.Replace(strings.Replace(templ, "ARCH", arch, 1), "OS", os, 1), "VARIANT", variant, 1),
						Platform: ocispec.Platform{
							OS:           os,
							Architecture: arch,
							Variant:      variant,
						},
					})
				}
				yamlInput := types.YAMLInput{
					Image:     target,
					Tags:      tags,
					Manifests: srcImages,
				}
				manifestType := types.Docker
				if c.String("type") == "oci" {
					manifestType = types.OCI
				}
				digest, length, err := registry.PushManifestList(c.String("username"), c.String("password"), yamlInput, c.Bool("ignore-missing"), c.Bool("insecure"), c.Bool("plain-http"), manifestType, c.String("docker-cfg"))
				if err != nil {
					logrus.Fatal(err)
				}
				fmt.Printf("Digest: %s %d\n", digest, length)

				return nil
			},
		},
	},
}

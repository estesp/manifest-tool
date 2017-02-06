package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/estesp/manifest-tool/docker"
)

var inspectCmd = cli.Command{
	Name:  "inspect",
	Usage: "inspect images on a registry",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "raw",
			Usage: "raw JSON output",
		},
	},
	Action: func(c *cli.Context) {

		name := c.Args().First()
		a := getAuthInfo(c)
		mfListDigest, imgInspect, _, err := docker.GetImageData(a, name)
		if err != nil {
			logrus.Fatal(err)
		}
		if c.Bool("raw") {
			out, err := json.Marshal(imgInspect)
			if err != nil {
				logrus.Fatal(err)
			}
			fmt.Println(string(out))
			return
		}
		// output basic informative details about the image
		if mfListDigest == "" {
			// this is a basic single manifest
			fmt.Printf("%s: manifest type: %s\n", name, imgInspect[0].MediaType)
			fmt.Printf("      Digest: %s\n", imgInspect[0].Digest)
			fmt.Printf("Architecture: %s\n", imgInspect[0].Architecture)
			fmt.Printf("          OS: %s\n", imgInspect[0].Os)
			fmt.Printf("    # Layers: %d\n", len(imgInspect[0].Layers))
			for i, digest := range imgInspect[0].Layers {
				fmt.Printf("      layer %d: digest = %s\n", i+1, digest)
			}
			return
		}

		// we have a manifest list so show the parent and all nested images
		fmt.Printf("Repository: %s\n", name)
		fmt.Printf("    Digest: %s\n", mfListDigest)
		fmt.Printf(" Manifests: %d\n", len(imgInspect))
		fmt.Printf("\n")

		for i, img := range imgInspect {
			fmt.Printf("%d    Mfst Type: %s\n", i+1, img.MediaType)
			fmt.Printf("%d       Digest: %s\n", i+1, img.Digest)
			fmt.Printf("%d  Mfst Length: %d\n", i+1, img.Size)
			fmt.Printf("%d     Platform:\n", i+1)
			fmt.Printf("%d           -      OS: %s\n", i+1, img.Platform.OS)
			// WINDOWS SUPPORT - NOT VENDORED YET fmt.Printf("%d           - OS Vers: %s\n", i+1, img.Platform.OSVersion)
			// WINDOWS SUPPORT - NOT VENDORED YET fmt.Printf("%d           - OS Feat: %s\n", i+1, img.Platform.OSFeatures)
			fmt.Printf("%d           -    Arch: %s\n", i+1, img.Platform.Architecture)
			fmt.Printf("%d           - Variant: %s\n", i+1, img.Platform.Variant)
			fmt.Printf("%d           - Feature: %s\n", i+1, strings.Join(img.Platform.Features, ","))
			fmt.Printf("%d     # Layers: %d\n", i+1, len(img.Layers))
			for j, digest := range img.Layers {
				fmt.Printf("         layer %d: digest = %s\n", j+1, digest)
			}
			fmt.Println()
		}
	},
}

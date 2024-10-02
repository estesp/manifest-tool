package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/estesp/manifest-tool/v2/pkg/registry"
	"github.com/estesp/manifest-tool/v2/pkg/store"
	"github.com/estesp/manifest-tool/v2/pkg/types"
	"github.com/estesp/manifest-tool/v2/pkg/util"

	"github.com/fatih/color"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var inspectCmd = &cli.Command{
	Name:  "inspect",
	Usage: "fetch image manifests in a container registry",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "raw",
			Usage: "raw JSON output",
		},
		&cli.BoolFlag{
			Name:  "expand-config",
			Usage: "expand image config content in raw JSON output",
		},
	},
	Action: func(c *cli.Context) error {

		name := c.Args().First()
		imageRef, err := util.ParseName(name)
		if err != nil {
			logrus.Fatal(err)
		}
		if _, ok := imageRef.(reference.NamedTagged); !ok {
			if _, ok := imageRef.(reference.Digested); !ok {
				logrus.Fatal("image reference must include a tag or a digest; manifest-tool does not default to 'latest'")
			}
		}

		if c.Bool("expand-config") && !c.Bool("raw") {
			logrus.Fatal("the --expand-config flag is only valid when used with --raw")
		}
		memoryStore := store.NewMemoryStore()
		err = util.CreateRegistryHost(imageRef, c.String("username"), c.String("password"), c.Bool("insecure"),
			c.Bool("plain-http"), c.String("docker-cfg"), false)
		if err != nil {
			return fmt.Errorf("error creating registry host configuration: %v", err)
		}

		descriptor, err := registry.FetchDescriptor(util.GetResolver(), memoryStore, imageRef)
		if err != nil {
			logrus.Error(err)
		}

		if c.Bool("raw") {
			out, err := generateRawJSON(name, descriptor, c.Bool("expand-config"), memoryStore)
			if err != nil {
				logrus.Fatal(err)
			}
			fmt.Println(string(out))
			return nil
		}
		_, db, _ := memoryStore.Get(descriptor)
		switch descriptor.MediaType {
		case ocispec.MediaTypeImageIndex, types.MediaTypeDockerSchema2ManifestList:
			// this is a multi-platform image descriptor; marshal to Index type
			var idx ocispec.Index
			if err := json.Unmarshal(db, &idx); err != nil {
				logrus.Fatal(err)
			}
			outputList(name, memoryStore, descriptor, idx)
		case ocispec.MediaTypeImageManifest, types.MediaTypeDockerSchema2Manifest:
			var man ocispec.Manifest
			if err := json.Unmarshal(db, &man); err != nil {
				logrus.Fatal(err)
			}
			_, cb, _ := memoryStore.Get(man.Config)
			var conf ocispec.Image
			if err := json.Unmarshal(cb, &conf); err != nil {
				logrus.Fatal(err)
			}
			outputImage(name, descriptor, man, conf)
		default:
			logrus.Errorf("Unknown descriptor type: %s", descriptor.MediaType)
		}

		return nil
	},
}

func outputList(name string, cs *store.MemoryStore, descriptor ocispec.Descriptor, index ocispec.Index) {
	var (
		yellow = color.New(color.Bold, color.FgYellow).SprintFunc()
		red    = color.New(color.Bold, color.FgRed).SprintFunc()
		blue   = color.New(color.Bold, color.FgBlue).SprintFunc()
		green  = color.New(color.Bold, color.FgGreen).SprintFunc()
	)
	fmt.Printf("Name:   %s (Type: %s)\n", green(name), green(descriptor.MediaType))
	fmt.Printf("Digest: %s\n", yellow(descriptor.Digest))

	outputStr := strings.Builder{}
	var attestations int
	for i, img := range index.Manifests {
		var attestationDetail string

		if aRefType, ok := img.Annotations["vnd.docker.reference.type"]; ok {
			if aRefType == "attestation-manifest" {
				attestations++
				attestationDetail = " (vnd.docker.reference.type=attestation-manifest)"
			}
		}
		outputStr.WriteString(fmt.Sprintf("[%d]     Type: %s%s\n", i+1, green(img.MediaType), green(attestationDetail)))
		outputStr.WriteString(fmt.Sprintf("[%d]   Digest: %s\n", i+1, yellow(img.Digest)))
		outputStr.WriteString(fmt.Sprintf("[%d]   Length: %s\n", i+1, blue(img.Size)))

		_, db, _ := cs.Get(img)
		switch img.MediaType {
		case ocispec.MediaTypeImageManifest, types.MediaTypeDockerSchema2Manifest:
			var man ocispec.Manifest
			if err := json.Unmarshal(db, &man); err != nil {
				logrus.Fatal(err)
			}
			if len(attestationDetail) > 0 {
				// only output info about the attestation info
				attestRef := img.Annotations["vnd.docker.reference.digest"]
				outputStr.WriteString(fmt.Sprintf("[%d]       >>> Attestation for digest: %s\n\n", i+1, yellow(attestRef)))
				continue
			}
			outputStr.WriteString(fmt.Sprintf("[%d] Platform:\n", i+1))
			outputStr.WriteString(fmt.Sprintf("[%d]    -      OS: %s\n", i+1, green(img.Platform.OS)))
			if img.Platform.OSVersion != "" {
				outputStr.WriteString(fmt.Sprintf("[%d]    - OS Vers: %s\n", i+1, green(img.Platform.OSVersion)))
			}
			if len(img.Platform.OSFeatures) > 0 {
				outputStr.WriteString(fmt.Sprintf("[%d]    - OS Feat: %s\n", i+1, green(img.Platform.OSFeatures)))
			}
			outputStr.WriteString(fmt.Sprintf("[%d]    -    Arch: %s\n", i+1, green(img.Platform.Architecture)))
			if img.Platform.Variant != "" {
				outputStr.WriteString(fmt.Sprintf("[%d]    - Variant: %s\n", i+1, green(img.Platform.Variant)))
			}
			outputStr.WriteString(fmt.Sprintf("[%d] # Layers: %s\n", i+1, red(len(man.Layers))))
			for j, layer := range man.Layers {
				outputStr.WriteString(fmt.Sprintf("     layer %s: digest = %s\n", red(fmt.Sprintf("%02d", j+1)), yellow(layer.Digest)))
				outputStr.WriteString(fmt.Sprintf("                 type = %s\n", green(layer.MediaType)))
			}
			outputStr.WriteString("\n")
		default:
			outputStr.WriteString(fmt.Sprintf("Unknown media type for further display: %s\n", img.MediaType))
		}
	}
	imageCount := len(index.Manifests) - attestations
	imageStr := "image"
	attestStr := "attestation"
	if imageCount > 1 {
		imageStr = "images"
	}
	if attestations > 1 {
		attestStr = "attestations"
	}
	fmt.Printf(" * Contains %s manifest references (%s %s, %s %s):\n", red(len(index.Manifests)),
		red(imageCount), imageStr, red(attestations), attestStr)
	fmt.Printf("%s", outputStr.String())
}

func outputImage(name string, descriptor ocispec.Descriptor, manifest ocispec.Manifest, config ocispec.Image) {
	var (
		yellow = color.New(color.Bold, color.FgYellow).SprintFunc()
		red    = color.New(color.Bold, color.FgRed).SprintFunc()
		blue   = color.New(color.Bold, color.FgBlue).SprintFunc()
		green  = color.New(color.Bold, color.FgGreen).SprintFunc()
	)
	fmt.Printf("Name: %s (Type: %s)\n", green(name), green(descriptor.MediaType))
	fmt.Printf("      Digest: %s\n", yellow(descriptor.Digest))
	fmt.Printf("        Size: %s\n", blue(descriptor.Size))
	fmt.Printf("          OS: %s\n", green(config.OS))
	fmt.Printf("        Arch: %s\n", green(config.Architecture))
	fmt.Printf("    # Layers: %s\n", red(len(manifest.Layers)))
	for i, layer := range manifest.Layers {
		fmt.Printf("      layer %s: digest = %s\n", red(fmt.Sprintf("%02d", i+1)), yellow(layer.Digest))
	}
}

// struct for modeling an index as raw JSON output in a format that
// includes the same content displayed in human-readable format
type indexJson struct {
	Name          string             `json:"name"`
	Digest        string             `json:"digest"`
	SchemaVersion int                `json:"schemaVersion"`
	MediaType     string             `json:"mediaType,omitempty"`
	Manifests     []ocispec.Manifest `json:"manifests"`
	Annotations   map[string]string  `json:"annotations,omitempty"`
}

// struct for modeling a manifest as raw JSON output in a format that
// includes the same content displayed in human-readable format
type manifestJson struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
	Os     string `json:"os,omitempty"`
	Arch   string `json:"architecture,omitempty"`
	ocispec.Manifest
}

type manifestConfigJson struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Name          string               `json:"name"`
	Digest        string               `json:"digest"`
	Os            string               `json:"os,omitempty"`
	Arch          string               `json:"architecture,omitempty"`
	MediaType     string               `json:"mediaType,omitempty"`
	Config        ocispec.Image        `json:"config"`
	Layers        []ocispec.Descriptor `json:"layers"`
	Subject       *ocispec.Descriptor  `json:"subject,omitempty"`
	Annotations   map[string]string    `json:"annotations,omitempty"`
}

func generateRawJSON(name string, descriptor ocispec.Descriptor, expandConfig bool, ms *store.MemoryStore) (string, error) {

	_, db, _ := ms.Get(descriptor)
	switch descriptor.MediaType {
	case ocispec.MediaTypeImageIndex, types.MediaTypeDockerSchema2ManifestList:
		// this is a multi-platform image descriptor; marshal to Index type
		var idx ocispec.Index
		if err := json.Unmarshal(db, &idx); err != nil {
			return "", err
		}
		indexJSON := indexJson{
			Name:          name,
			Digest:        descriptor.Digest.String(),
			SchemaVersion: idx.SchemaVersion,
			MediaType:     idx.MediaType,
			Annotations:   idx.Annotations,
		}
		for _, m := range idx.Manifests {
			_, man, _ := ms.Get(m)
			switch m.MediaType {
			case ocispec.MediaTypeImageManifest, types.MediaTypeDockerSchema2Manifest:
				var image ocispec.Manifest
				if err := json.Unmarshal(man, &image); err != nil {
					return "", err
				}
				indexJSON.Manifests = append(indexJSON.Manifests, image)
			default:
				return "", fmt.Errorf("unknown media type for further display: %s", m.MediaType)
			}
		}
		b, err := json.MarshalIndent(indexJSON, "", "    ")
		if err != nil {
			return "", err
		}
		return string(b), nil
	case ocispec.MediaTypeImageManifest, types.MediaTypeDockerSchema2Manifest:
		var man ocispec.Manifest
		if err := json.Unmarshal(db, &man); err != nil {
			return "", err
		}
		_, cb, _ := ms.Get(man.Config)
		var conf ocispec.Image
		if err := json.Unmarshal(cb, &conf); err != nil {
			return "", err
		}
		var rawJSON interface{}
		if !expandConfig {
			rawJSON = manifestJson{
				Name:     name,
				Digest:   descriptor.Digest.String(),
				Os:       conf.OS,
				Arch:     conf.Architecture,
				Manifest: man,
			}
		} else {
			rawJSON = manifestConfigJson{
				Name:          name,
				Digest:        descriptor.Digest.String(),
				Os:            conf.OS,
				Arch:          conf.Architecture,
				Config:        conf,
				Layers:        man.Layers,
				SchemaVersion: man.SchemaVersion,
				MediaType:     man.MediaType,
				Annotations:   man.Annotations,
				Subject:       man.Subject,
			}
		}
		b, err := json.MarshalIndent(rawJSON, "", "    ")
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return "", fmt.Errorf("unknown descriptor type: %s", descriptor.MediaType)
	}
}

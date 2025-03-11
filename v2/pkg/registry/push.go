package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/estesp/manifest-tool/v2/pkg/store"
	"github.com/estesp/manifest-tool/v2/pkg/types"
	"github.com/estesp/manifest-tool/v2/pkg/util"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

func PushManifestList(username, password string, input types.YAMLInput, ignoreMissing, insecure, plainHttp bool, manifestType types.ManifestType, configDir string) (hash string, length int, err error) {
	// resolve the target image reference for the combined manifest list/index
	if manifestType == types.Docker && len(input.Annotations) > 0 {
		return hash, length, fmt.Errorf("manifest list (Docker media type) does not support annotations; use --type oci instead")
	}
	targetRef, err := reference.ParseNormalizedNamed(input.Image)
	if err != nil {
		return hash, length, fmt.Errorf("error parsing name for manifest list (%s): %v", input.Image, err)
	}

	err = util.CreateRegistryHost(targetRef, username, password, insecure, plainHttp, configDir, true)
	if err != nil {
		return hash, length, fmt.Errorf("error creating registry host configuration: %v", err)
	}
	manifestList := types.ManifestList{
		Name:        input.Image,
		Reference:   targetRef,
		Resolver:    util.GetResolver(),
		Type:        manifestType,
		Annotations: input.Annotations,
	}
	// create an in-memory store for OCI descriptors and content used during the push operation
	memoryStore := store.NewMemoryStore()

	// collect descriptors for images and attestations as we walk the included images
	var (
		manifestDescriptors    []types.Manifest
		attestationDescriptors []types.Manifest
		platforms              map[string]ocispec.Descriptor
	)

	logrus.Info("Retrieving digests of member images")
	for _, img := range input.Manifests {
		ref, err := util.ParseName(img.Image)
		if err != nil {
			return hash, length, fmt.Errorf("unable to parse image reference: %s: %v", img.Image, err)
		}
		if reference.Domain(targetRef) != reference.Domain(ref) {
			return hash, length, fmt.Errorf("source image (%s) registry does not match target image (%s) registry", ref, targetRef)
		}
		descriptor, err := FetchDescriptor(util.GetResolver(), memoryStore, ref)
		if err != nil {
			if ignoreMissing {
				logrus.Warnf("Couldn't access image '%q'. Skipping due to 'ignore missing' configuration.", img.Image)
				continue
			}
			return hash, length, fmt.Errorf("inspect of image %q failed with error: %v", img.Image, err)
		}

		// Check that only member images of type OCI manifest or Docker v2.2 manifest are included
		switch descriptor.MediaType {
		case ocispec.MediaTypeImageIndex, types.MediaTypeDockerSchema2ManifestList:
			// check if the index simply has a single image and that other index entries are attestation manifests
			desc, attestDesc := getImagesFromIndex(descriptor, memoryStore)
			var pushRef bool
			if reference.Path(ref) != reference.Path(targetRef) {
				pushRef = true
			}
			for _, d := range desc {
				man := types.Manifest{
					Descriptor: d,
					PushRef:    pushRef,
				}
				manifestDescriptors = append(manifestDescriptors, man)
			}
			for _, d := range attestDesc {
				man := types.Manifest{
					Descriptor: d,
					PushRef:    pushRef,
				}
				attestationDescriptors = append(attestationDescriptors, man)
			}
		case ocispec.MediaTypeImageManifest, types.MediaTypeDockerSchema2Manifest:
			var (
				man       ocispec.Manifest
				imgConfig types.Image
				pushRef   bool
			)
			// finalize the platform object that will be used to push with this manifest
			_, db, _ := memoryStore.Get(descriptor)
			if err := json.Unmarshal(db, &man); err != nil {
				return hash, length, fmt.Errorf("could not unmarshal manifest object from descriptor for image '%s': %v", img.Image, err)
			}
			_, cb, _ := memoryStore.Get(man.Config)
			if err := json.Unmarshal(cb, &imgConfig); err != nil {
				return hash, length, fmt.Errorf("could not unmarshal config object from descriptor for image '%s': %v", img.Image, err)
			}
			descriptor.Platform, err = resolvePlatform(descriptor, img, imgConfig)
			if err != nil {
				return hash, length, fmt.Errorf("unable to create platform object for manifest %s: %v", descriptor.Digest.String(), err)
			}
			if reference.Path(ref) != reference.Path(targetRef) {
				pushRef = true
			}
			manifestDescriptors = append(manifestDescriptors, types.Manifest{
				Descriptor: descriptor,
				PushRef:    pushRef,
			})
		default:
			return hash, length, fmt.Errorf("cannot include unknown media type '%s' in a manifest list/index push", descriptor.MediaType)
		}
	}

	platforms = make(map[string]ocispec.Descriptor)

	// add image manifests to final index/manifestlist
	for _, manifest := range manifestDescriptors {
		// first make sure we haven't already encountered an image with this platform
		platStr := getPlatformString(manifest.Descriptor.Platform)
		if otherDesc, ok := platforms[platStr]; ok {
			return hash, length, fmt.Errorf("cannot include two manifests with the same platform; digest %s already provides platform %s (this digest: %s)", otherDesc.Digest.String(),
				platStr, manifest.Descriptor.Digest.String())
		}
		platforms[platStr] = manifest.Descriptor

		var man ocispec.Manifest
		_, db, _ := memoryStore.Get(manifest.Descriptor)
		if err := json.Unmarshal(db, &man); err != nil {
			return hash, length, fmt.Errorf("could not unmarshal manifest object from descriptor '%s': %v", manifest.Descriptor.Digest.String(), err)
		}
		// set labels for handling distribution source to get automatic cross-repo blob mounting for the layers
		info, _ := memoryStore.Info(context.TODO(), manifest.Descriptor.Digest)
		for _, layer := range man.Layers {
			// only need to handle cross-repo blob mount for distributable layer types
			if skippable(layer.MediaType) {
				continue
			}
			info.Digest = layer.Digest
			if _, err := memoryStore.Update(context.TODO(), info, ""); err != nil {
				logrus.Warnf("couldn't update in-memory store labels for %v: %v", info.Digest, err)
			}
		}
		manifestList.Manifests = append(manifestList.Manifests, manifest)
	}

	// add attestations to final index/manifestlist
	for _, attestation := range attestationDescriptors {
		_, db, _ := memoryStore.Get(attestation.Descriptor)
		var man ocispec.Manifest
		if err := json.Unmarshal(db, &man); err != nil {
			return hash, length, fmt.Errorf("could not unmarshal attestation object from descriptor '%s': %v", attestation.Descriptor.Digest.String(), err)
		}
		info, _ := memoryStore.Info(context.TODO(), attestation.Descriptor.Digest)
		for _, layer := range man.Layers {
			// only need to handle cross-repo blob mount for distributable layer types
			if skippable(layer.MediaType) {
				continue
			}
			info.Digest = layer.Digest
			if _, err := memoryStore.Update(context.TODO(), info, ""); err != nil {
				logrus.Warnf("couldn't update in-memory store labels for %v: %v", info.Digest, err)
			}
		}
		manifestList.Manifests = append(manifestList.Manifests, attestation)
	}

	if ignoreMissing && len(manifestList.Manifests) == 0 {
		// we need to verify we at least have one valid entry in the list
		// otherwise our manifest list will be totally empty
		return hash, length, fmt.Errorf("all entries were skipped due to missing source image references; no manifest list to push")
	}

	return Push(manifestList, input.Tags, memoryStore)
}

func resolvePlatform(descriptor ocispec.Descriptor, img types.ManifestEntry, imgConfig types.Image) (*ocispec.Platform, error) {
	platform := &img.Platform
	// fill os/arch from inspected image if not specified in input YAML
	if platform.OS == "" && platform.Architecture == "" {
		// prefer a full platform object, if one is already available (and appears to have meaningful content)
		if descriptor.Platform != nil && (descriptor.Platform.OS != "" || descriptor.Platform.Architecture != "") {
			platform = descriptor.Platform
		} else if imgConfig.OS != "" || imgConfig.Architecture != "" {
			platform.OS = imgConfig.OS
			platform.Architecture = imgConfig.Architecture
		}
	}
	// if Variant is specified in the origin image but not the descriptor or YAML, bubble it up
	if imgConfig.Variant != "" && platform.Variant == "" {
		platform.Variant = imgConfig.Variant
	}
	// Windows: if the origin image has OSFeature and/or OSVersion information, and
	// these values were not specified in the creation YAML, then
	// retain the origin values in the Platform definition for the manifest list:
	if imgConfig.OSVersion != "" && platform.OSVersion == "" {
		platform.OSVersion = imgConfig.OSVersion
	}
	if len(imgConfig.OSFeatures) > 0 && len(platform.OSFeatures) == 0 {
		platform.OSFeatures = imgConfig.OSFeatures
	}

	// validate os/arch input
	if !util.IsValidOSArch(platform.OS, platform.Architecture, platform.Variant) {
		return nil, fmt.Errorf("manifest entry for image %s has unsupported os/arch or os/arch/variant combination: %s/%s/%s", img.Image, platform.OS, platform.Architecture, platform.Variant)
	}
	return platform, nil
}

func skippable(mediaType string) bool {
	// skip foreign/non-distributable layers
	if strings.Index(mediaType, "foreign") > 0 || strings.Index(mediaType, "nondistributable") > 0 {
		return true
	}
	// skip manifests (OCI or Dockerv2) as they are already handled on push references code
	switch mediaType {
	case ocispec.MediaTypeImageManifest, types.MediaTypeDockerSchema2Manifest:
		return true
	}
	return false
}

func isAttestationManifest(desc ocispec.Descriptor) bool {
	if aRefType, ok := desc.Annotations["vnd.docker.reference.type"]; ok {
		if aRefType == "attestation-manifest" {
			return true
		}
	}
	return false
}

func getImagesFromIndex(desc ocispec.Descriptor, ms *store.MemoryStore) ([]ocispec.Descriptor, []ocispec.Descriptor) {
	var (
		manifests    []ocispec.Descriptor
		attestations []ocispec.Descriptor
	)
	_, db, _ := ms.Get(desc)
	var index ocispec.Index
	if err := json.Unmarshal(db, &index); err != nil {
		logrus.Errorf("could not unmarshal index from descriptor '%s': %v", desc.Digest.String(), err)
		return manifests, attestations
	}
	for _, man := range index.Manifests {
		if isAttestationManifest(man) {
			attestations = append(attestations, man)
		} else {
			manifests = append(manifests, man)
		}
	}
	return manifests, attestations
}

func getPlatformString(platform *ocispec.Platform) string {
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		platform.Architecture,
		platform.OS,
		platform.Variant,
		platform.OSVersion,
		strings.Join(platform.OSFeatures, "."))
}

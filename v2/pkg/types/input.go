package types

import ocispec "github.com/opencontainers/image-spec/specs-go/v1"

// YAMLInput contains the parsed yaml fields from the push
// command of manifest-tool
type YAMLInput struct {
	Image       string
	Tags        []string
	Manifests   []ManifestEntry
	Annotations map[string]string
}

// ManifestEntry contains an image reference and it's corresponding OCI
// platform definition (OS/Arch/Variant)
type ManifestEntry struct {
	Image    string
	Platform ocispec.Platform
}

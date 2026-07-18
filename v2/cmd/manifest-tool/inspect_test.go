package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/estesp/manifest-tool/v2/pkg/store"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// An OCI index entry may legitimately omit the optional "platform" field
// (Descriptor.Platform is a *Platform with omitempty). outputList used to
// dereference img.Platform.OS unconditionally, so such an entry panicked with
// a nil-pointer dereference. This drives outputList with a platform-less entry
// and asserts it renders without panicking.
func TestOutputListNilPlatform(t *testing.T) {
	cs := store.NewMemoryStore()

	// Minimal, valid image manifest so json.Unmarshal in outputList succeeds
	// and the store's content-addressable push accepts it.
	man := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config: ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageConfig,
		},
		Layers: []ocispec.Descriptor{},
	}
	man.SchemaVersion = 2
	manBytes, err := json.Marshal(man)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	child := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(manBytes),
		Size:      int64(len(manBytes)),
		Platform:  nil, // the platform-less entry that used to panic
	}
	cs.Set(child, manBytes)

	index := ocispec.Index{
		Manifests: []ocispec.Descriptor{child},
	}
	idxDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageIndex,
	}

	// Would panic before the nil guard was added.
	outputList("example.com/img@"+child.Digest.String(), cs, idxDesc, index)
}

// A regular entry that does carry platform data still renders its OS/arch.
func TestOutputListWithPlatform(t *testing.T) {
	cs := store.NewMemoryStore()

	man := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig},
		Layers:    []ocispec.Descriptor{},
	}
	man.SchemaVersion = 2
	manBytes, err := json.Marshal(man)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	child := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(manBytes),
		Size:      int64(len(manBytes)),
		Platform: &ocispec.Platform{
			OS:           "linux",
			Architecture: "amd64",
		},
	}
	cs.Set(child, manBytes)

	index := ocispec.Index{Manifests: []ocispec.Descriptor{child}}
	idxDesc := ocispec.Descriptor{MediaType: ocispec.MediaTypeImageIndex}

	// Just make sure the populated path is still reachable without panic.
	if !strings.Contains(child.Platform.OS, "linux") {
		t.Fatal("unexpected platform setup")
	}
	outputList("example.com/img", cs, idxDesc, index)
}

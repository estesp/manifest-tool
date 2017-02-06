package docker

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/opencontainers/go-digest"

	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"

	"github.com/estesp/manifest-tool/types"
)

// we will store up a list of blobs we must ask the registry
// to cross-mount into our target namespace
type blobMount struct {
	FromRepo string
	Digest   string
}

// if we have mounted blobs referenced from manifests from
// outside the target repository namespace we will need to
// push them to our target's repo as they will be references
// from the final manifest list object we push
type manifestPush struct {
	Name      string
	Digest    string
	JSONBytes []byte
	MediaType string
}

// PutManifestList takes an authentication variable and a yaml spec struct and pushes an image list based on the spec
func PutManifestList(a *types.AuthInfo, yamlInput types.YAMLInput) (string, error) {
	var (
		manifestList      manifestlist.ManifestList
		blobMountRequests []blobMount
		manifestRequests  []manifestPush
	)

	// process the final image name reference for the manifest list
	targetRef, err := reference.ParseNamed(yamlInput.Image)
	if err != nil {
		return "", fmt.Errorf("Error parsing name for manifest list (%s): %v", yamlInput.Image, err)
	}
	targetRepo, err := registry.ParseRepositoryInfo(targetRef)
	if err != nil {
		return "", fmt.Errorf("Error parsing repository name for manifest list (%s): %v", yamlInput.Image, err)
	}
	targetEndpoint, repoName, err := setupRepo(targetRepo)
	if err != nil {
		return "", fmt.Errorf("Error setting up repository endpoint and references for %q: %v", targetRef, err)
	}

	// Now create the manifest list payload by looking up the manifest schemas
	// for the constituent images:
	logrus.Info("Retrieving digests of images...")
	for _, img := range yamlInput.Manifests {
		// validate os/arch input
		if !isValidOSArch(img.Platform.OS, img.Platform.Architecture) {
			return "", fmt.Errorf("Manifest entry for image %s has unsupported os/arch combination: %s/%s", img.Image, img.Platform.OS, img.Platform.Architecture)
		}
		_, mfstData, repoInfo, err := GetImageData(a, img.Image)
		if err != nil {
			return "", fmt.Errorf("Inspect of image %q failed with error: %v", img.Image, err)
		}
		if repoInfo.Hostname() != targetRepo.Hostname() {
			return "", fmt.Errorf("Cannot use source images from a different registry than the target image: %s != %s", repoInfo.Hostname(), targetRepo.Hostname())
		}
		if len(mfstData) > 1 {
			// too many responses--can only happen if a manifest list was returned for the name lookup
			return "", fmt.Errorf("You specified a manifest list entry from a digest that points to a current manifest list. Manifest lists do not allow recursion.")
		}
		// the non-manifest list case will always have exactly one manifest response
		imgMfst := mfstData[0]

		manifest := manifestlist.ManifestDescriptor{
			Platform: img.Platform,
		}
		manifest.Descriptor.Digest, err = digest.Parse(imgMfst.Digest)
		manifest.Size = imgMfst.Size
		manifest.MediaType = imgMfst.MediaType

		if err != nil {
			return "", fmt.Errorf("Digest parse of image %q failed with error: %v", img.Image, err)
		}
		logrus.Infof("Image %q is digest %s; size: %d", img.Image, imgMfst.Digest, imgMfst.Size)

		// if this image is in a different repo, we need to add the layer/blob digests to the list of
		// requested blob mounts (cross-repository push) before pushing the manifest list
		if repoName != repoInfo.RemoteName() {
			logrus.Debugf("Adding layers of %q to blob mount requests", img.Image)
			for _, layer := range imgMfst.Layers {
				blobMountRequests = append(blobMountRequests, blobMount{FromRepo: repoInfo.RemoteName(), Digest: layer})
			}
			// also must add the manifest to be pushed in the target namespace
			logrus.Debugf("Adding manifest %q -> to be pushed to %q as a manifest reference", repoInfo.RemoteName(), repoName)
			manifestRequests = append(manifestRequests, manifestPush{
				Name:      repoInfo.RemoteName(),
				Digest:    imgMfst.Digest,
				JSONBytes: imgMfst.CanonicalJSON,
				MediaType: imgMfst.MediaType,
			})
		}
		manifestList.Manifests = append(manifestList.Manifests, manifest)
	}

	// Set the schema version
	manifestList.Versioned = manifestlist.SchemaVersion

	urlBuilder, err := v2.NewURLBuilderFromString(targetEndpoint.URL.String(), false)
	if err != nil {
		return "", fmt.Errorf("Can't create URL builder from endpoint (%s): %v", targetEndpoint.URL.String(), err)
	}
	pushURL, err := createManifestURLFromRef(targetRef, urlBuilder)
	if err != nil {
		return "", fmt.Errorf("Error setting up repository endpoint and references for %q: %v", targetRef, err)
	}
	logrus.Debugf("Manifest list push url: %s", pushURL)

	deserializedManifestList, err := manifestlist.FromDescriptors(manifestList.Manifests)
	if err != nil {
		return "", fmt.Errorf("Cannot deserialize manifest list: %v", err)
	}
	mediaType, p, err := deserializedManifestList.Payload()
	logrus.Debugf("mediaType of manifestList: %s", mediaType)
	if err != nil {
		return "", fmt.Errorf("Cannot retrieve payload for HTTP PUT of manifest list: %v", err)

	}
	putRequest, err := http.NewRequest("PUT", pushURL, bytes.NewReader(p))
	if err != nil {
		return "", fmt.Errorf("HTTP PUT request creation failed: %v", err)
	}
	putRequest.Header.Set("Content-Type", mediaType)

	httpClient, err := getHTTPClient(a, targetRepo, targetEndpoint, repoName)
	if err != nil {
		return "", fmt.Errorf("Failed to setup HTTP client to repository: %v", err)
	}

	// before we push the manifest list, if we have any blob mount requests, we need
	// to ask the registry to mount those blobs in our target so they are available
	// as references
	if err := mountBlobs(httpClient, urlBuilder, targetRef, blobMountRequests); err != nil {
		return "", fmt.Errorf("Couldn't mount blobs for cross-repository push: %v", err)
	}

	// we also must push any manifests that are referenced in the manifest list into
	// the target namespace
	if err := pushReferences(httpClient, urlBuilder, targetRef, manifestRequests); err != nil {
		return "", fmt.Errorf("Couldn't push manifests referenced in our manifest list: %v", err)
	}

	resp, err := httpClient.Do(putRequest)
	if err != nil {
		return "", fmt.Errorf("V2 registry PUT of manifest list failed: %v", err)
	}
	defer resp.Body.Close()

	if statusSuccess(resp.StatusCode) {
		dgstHeader := resp.Header.Get("Docker-Content-Digest")
		dgst, err := digest.Parse(dgstHeader)
		if err != nil {
			return "", err
		}
		return string(dgst), nil
	}
	return "", fmt.Errorf("Registry push unsuccessful: response %d: %s", resp.StatusCode, resp.Status)
}

func getHTTPClient(a *types.AuthInfo, repoInfo *registry.RepositoryInfo, endpoint registry.APIEndpoint, repoName string) (*http.Client, error) {
	// get the http transport, this will be used in a client to upload manifest
	// TODO - add separate function get client
	base := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     endpoint.TLSConfig,
		DisableKeepAlives:   true,
	}
	authConfig, err := getAuthConfig(a, repoInfo.Index)
	if err != nil {
		return nil, fmt.Errorf("Cannot retrieve authconfig: %v", err)
	}
	modifiers := registry.DockerHeaders(dockerversion.DockerUserAgent(nil), http.Header{})
	authTransport := transport.NewTransport(base, modifiers...)
	challengeManager, _, err := registry.PingV2Registry(endpoint.URL, authTransport)
	if err != nil {
		return nil, fmt.Errorf("Ping of V2 registry failed: %v", err)
	}
	if authConfig.RegistryToken != "" {
		passThruTokenHandler := &existingTokenHandler{token: authConfig.RegistryToken}
		modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, passThruTokenHandler))
	} else {
		creds := dumbCredentialStore{auth: &authConfig}
		tokenHandler := auth.NewTokenHandler(authTransport, creds, repoName, "*")
		basicHandler := auth.NewBasicHandler(creds)
		modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler))
	}
	tr := transport.NewTransport(base, modifiers...)

	httpClient := &http.Client{
		Transport:     tr,
		CheckRedirect: checkHTTPRedirect,
	}
	return httpClient, nil
}

func createManifestURLFromRef(targetRef reference.Named, urlBuilder *v2.URLBuilder) (string, error) {
	// get rid of hostname so the target URL is constructed properly
	_, name := splitHostname(targetRef.String())
	targetRef, err := reference.ParseNamed(name)
	if err != nil {
		return "", fmt.Errorf("Can't parse target image repository name from reference: %v", err)
	}

	// Set the tag to latest, if no tag found in YAML
	if _, isTagged := targetRef.(reference.NamedTagged); !isTagged {
		targetRef, err = reference.WithTag(targetRef, reference.DefaultTag)
		if err != nil {
			return "", fmt.Errorf("Error adding default tag to target repository name: %v", err)
		}
	} else {
		tagged, _ := targetRef.(reference.NamedTagged)
		targetRef, err = reference.WithTag(targetRef, tagged.Tag())
		if err != nil {
			return "", fmt.Errorf("Error referencing specified tag to target repository name: %v", err)
		}
	}

	manifestURL, err := urlBuilder.BuildManifestURL(targetRef)
	if err != nil {
		return "", fmt.Errorf("Failed to build manifest URL from target reference: %v", err)
	}
	return manifestURL, nil
}

func setupRepo(repoInfo *registry.RepositoryInfo) (registry.APIEndpoint, string, error) {

	options := registry.ServiceOptions{}
	options.InsecureRegistries = append(options.InsecureRegistries, "0.0.0.0/0")
	registryService := registry.NewService(options)

	endpoints, err := registryService.LookupPushEndpoints(repoInfo.Hostname())
	if err != nil {
		return registry.APIEndpoint{}, "", err
	}
	logrus.Debugf("endpoints: %v", endpoints)
	// take highest priority endpoint
	endpoint := endpoints[0]

	repoName := repoInfo.FullName()
	// If endpoint does not support CanonicalName, use the RemoteName instead
	if endpoint.TrimHostname {
		repoName = repoInfo.RemoteName()
		logrus.Debugf("repoName: %v", repoName)
	}
	return endpoint, repoName, nil
}

func pushReferences(httpClient *http.Client, urlBuilder *v2.URLBuilder, ref reference.Named, manifests []manifestPush) error {
	// for each referenced manifest object in the manifest list (that is outside of our current repo/name)
	// we need to push by digest the manifest so that it is added as a valid reference in the current
	// repo. This will allow us to push the manifest list properly later and have all valid references.
	for _, manifest := range manifests {
		dgst, err := digest.Parse(manifest.Digest)
		if err != nil {
			return fmt.Errorf("Error parsing manifest digest (%s) for referenced manifest %q: %v", manifest.Digest, manifest.Name, err)
		}
		targetRef, err := reference.WithDigest(ref, dgst)
		if err != nil {
			return fmt.Errorf("Error creating manifest digest target for referenced manifest %q: %v", manifest.Name, err)
		}
		pushURL, err := urlBuilder.BuildManifestURL(targetRef)
		if err != nil {
			return fmt.Errorf("Error setting up manifest push URL for manifest references for %q: %v", manifest.Name, err)
		}
		logrus.Debugf("manifest reference push URL: %s", pushURL)

		pushRequest, err := http.NewRequest("PUT", pushURL, bytes.NewReader(manifest.JSONBytes))
		if err != nil {
			return fmt.Errorf("HTTP PUT request creation for manifest reference push failed: %v", err)
		}
		pushRequest.Header.Set("Content-Type", manifest.MediaType)
		resp, err := httpClient.Do(pushRequest)
		if err != nil {
			return fmt.Errorf("PUT of manifest reference failed: %v", err)
		}

		resp.Body.Close()
		if !statusSuccess(resp.StatusCode) {
			return fmt.Errorf("Referenced manifest push unsuccessful: response %d: %s", resp.StatusCode, resp.Status)
		}
		dgstHeader := resp.Header.Get("Docker-Content-Digest")
		dgstResult, err := digest.Parse(dgstHeader)
		if err != nil {
			return fmt.Errorf("Couldn't parse pushed manifest digest response: %v", err)
		}
		if string(dgstResult) != manifest.Digest {
			return fmt.Errorf("Pushed referenced manifest received a different digest: expected %s, got %s", manifest.Digest, string(dgst))
		}
		logrus.Debugf("referenced manifest %q pushed; digest matches: %s", manifest.Name, string(dgst))
	}
	return nil
}

func mountBlobs(httpClient *http.Client, urlBuilder *v2.URLBuilder, ref reference.Named, blobsRequested []blobMount) error {
	// get rid of hostname so the target URL is constructed properly
	_, name := splitHostname(ref.String())
	targetRef, _ := reference.ParseNamed(name)

	for _, blob := range blobsRequested {
		// create URL request
		url, err := urlBuilder.BuildBlobUploadURL(targetRef, url.Values{"from": {blob.FromRepo}, "mount": {blob.Digest}})
		if err != nil {
			return fmt.Errorf("Failed to create blob mount URL: %v", err)
		}
		mountRequest, err := http.NewRequest("POST", url, nil)
		if err != nil {
			return fmt.Errorf("HTTP POST request creation for blob mount failed: %v", err)
		}
		mountRequest.Header.Set("Content-Length", "0")
		resp, err := httpClient.Do(mountRequest)
		if err != nil {
			return fmt.Errorf("V2 registry POST of blob mount failed: %v", err)
		}

		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("Blob mount failed to url %s: HTTP status %d", url, resp.StatusCode)
		}
		logrus.Debugf("Mount of blob %s succeeded, location: %q", blob.Digest, resp.Header.Get("Location"))
	}
	return nil
}

func statusSuccess(status int) bool {
	return status >= 200 && status <= 399
}

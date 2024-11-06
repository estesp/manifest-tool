package util

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/credentials"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/pkg/homedir"
	"github.com/sirupsen/logrus"
)

var (
	configDir     = os.Getenv("DOCKER_CONFIG")
	configFileDir = ".docker"
	registryHost  docker.RegistryHost
)

func CreateRegistryHost(imageRef reference.Named, username, password string, insecure, plainHTTP bool, dockerConfigPath string, pushOp bool) error {

	hostname, _ := splitHostname(imageRef.String())
	if hostname == "docker.io" {
		hostname = "registry-1.docker.io"
	}
	registryHost = docker.RegistryHost{
		Host:         hostname,
		Scheme:       "https",
		Path:         "/v2",
		Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve,
	}
	if pushOp {
		registryHost.Capabilities |= docker.HostCapabilityPush
	}

	client := http.DefaultClient

	if insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}
	registryHost.Client = client

	if plainHTTP {
		registryHost.Scheme = "http"
	}

	credFunc := func(hostName string) (string, string, error) {
		if username != "" || password != "" {
			return username, password, nil
		}
		var (
			err error
			cfg *configfile.ConfigFile
		)
		if dockerConfigPath == "" || dockerConfigPath == configDir {
			cfg, err = config.Load(configDir)
			if err != nil {
				logrus.Warnf("unable to load default Docker auth config: %v", err)
			}
		} else {
			cfg = configfile.New(dockerConfigPath)
			if _, err := os.Stat(dockerConfigPath); err == nil {
				file, err := os.Open(dockerConfigPath)
				if err != nil {
					return "", "", fmt.Errorf("can't load docker config file %s: %w", dockerConfigPath, err)
				}
				defer file.Close()
				if err := cfg.LoadFromReader(file); err != nil {
					return "", "", fmt.Errorf("can't read and parse docker config file %s: %v", dockerConfigPath, err)
				}
			} else if !os.IsNotExist(err) {
				return "", "", fmt.Errorf("unable to open docker config file %s: %v", dockerConfigPath, err)
			}
		}
		if !cfg.ContainsAuth() {
			cfg.CredentialsStore = credentials.DetectDefaultStore(cfg.CredentialsStore)
		}
		hostname := resolveHostname(hostName)
		auth, err := cfg.GetAuthConfig(hostname)
		if err != nil {
			return "", "", err
		}
		if auth.IdentityToken != "" {
			return "", auth.IdentityToken, nil
		}
		return auth.Username, auth.Password, nil

	}
	registryHost.Authorizer = docker.NewDockerAuthorizer(docker.WithAuthCreds(credFunc))

	return nil
}

func GetResolver() remotes.Resolver {

	opts := docker.ResolverOptions{
		Hosts: getHosts,
	}
	return docker.NewResolver(opts)
}

func getHosts(name string) ([]docker.RegistryHost, error) {
	return []docker.RegistryHost{registryHost}, nil
}

// resolveHostname resolves Docker specific hostnames
func resolveHostname(hostname string) string {
	if strings.HasSuffix(hostname, "docker.io") {
		// Docker's `config.json` uses index.docker.io as the reference
		return LegacyDefaultHostname
	}
	return hostname
}

func init() {
	if configDir == "" {
		configDir = filepath.Join(homedir.Get(), configFileDir)
	}
}

func ConfigDir() string {
	return configDir
}

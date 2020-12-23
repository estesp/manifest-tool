package util

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/config"
	auth "github.com/deislabs/oras/pkg/auth/docker"
	"github.com/sirupsen/logrus"
)

func NewResolver(username, password string, insecure, plainHTTP bool, configs ...string) remotes.Resolver {
	opts := docker.ResolverOptions{
		PlainHTTP: plainHTTP,
	}
	client := http.DefaultClient
	if insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}
	opts.Client = client

	if username != "" || password != "" {
		opts.Credentials = func(hostName string) (string, string, error) {
			return username, password, nil
		}
		opts.Hosts = config.ConfigureHosts(context.Background(), config.HostOptions{
			Credentials: func(host string) (string, string, error) {
				// If host doesn't match...
				// Only one host
				return username, password, nil
			},
			DefaultTLS: &tls.Config{
				InsecureSkipVerify: insecure,
			},
		})
		return docker.NewResolver(opts)
	}
	cli, err := auth.NewClient(configs...)
	if err != nil {
		logrus.Warnf("Error loading auth file: %v", err)
	}
	resolver, err := cli.Resolver(context.Background(), client, plainHTTP)
	if err != nil {
		logrus.Warnf("Error loading resolver: %v", err)
		resolver = docker.NewResolver(opts)
	}
	return resolver
}

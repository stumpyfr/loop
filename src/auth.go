package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

type artifactRepository interface {
	Resolve(context.Context, string) (ocispec.Descriptor, error)
	Fetch(context.Context, ocispec.Descriptor) (io.ReadCloser, error)
	Push(context.Context, ocispec.Descriptor, io.Reader) error
	Tag(context.Context, ocispec.Descriptor, string) error
}

var openRepository = func(opts options) (artifactRepository, string, error) {
	repo, err := newRemoteRepository(opts)
	if err != nil {
		return nil, "", err
	}
	return repo, repo.Reference.Reference, nil
}

func newRemoteRepository(opts options) (*remote.Repository, error) {
	repo, err := remote.NewRepository(opts.targetRef)
	if err != nil {
		return nil, fmt.Errorf("create remote repository: %w", err)
	}
	credential, err := registryCredential(opts.registry)
	if err != nil {
		return nil, err
	}
	repo.Client = &auth.Client{
		Client:     http.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: credential,
	}
	return repo, nil
}

func registryCredential(registry string) (auth.CredentialFunc, error) {
	if strings.EqualFold(strings.TrimSuffix(registry, "/"), defaultRegistry) {
		if cred, ok := ghcrEnvCredential(); ok {
			return auth.StaticCredential(defaultRegistry, cred), nil
		}
	}

	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return nil, fmt.Errorf("load docker credentials: %w", err)
	}
	return credentials.Credential(store), nil
}

func ghcrEnvCredential() (auth.Credential, bool) {
	token := strings.TrimSpace(os.Getenv("GHCR_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	if token == "" {
		return auth.EmptyCredential, false
	}

	username := strings.TrimSpace(os.Getenv("GHCR_USERNAME"))
	if username == "" {
		username = strings.TrimSpace(os.Getenv("GITHUB_ACTOR"))
	}
	if username == "" {
		return auth.EmptyCredential, false
	}

	return auth.Credential{
		Username: username,
		Password: token,
	}, true
}

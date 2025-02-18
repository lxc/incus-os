package providers

import (
	"context"
	"errors"

	ghapi "github.com/google/go-github/v68/github"
)

var errNotImplemented = errors.New("not implemented")

// The Github provider.
type github struct {
	gh           *ghapi.Client
	organization string
	repository   string

	config map[string]string
}

func (p *github) load(_ context.Context) error {
	// Setup the Github client.
	p.gh = ghapi.NewClient(nil)

	// Fixed configuration for now.
	p.organization = "lxc"
	p.repository = "incus-os"

	return nil
}

func (p *github) GetOSUpdate(_ context.Context) (OSUpdate, error) {
	update := githubOSUpdate{
		gh:      p.gh,
		version: "unknown",
	}

	return &update, nil
}

func (p *github) GetApplication(_ context.Context, name string) (Application, error) {
	app := githubApplication{
		gh:      p.gh,
		name:    name,
		version: "unknown",
	}

	return &app, nil
}

// An application from the Github provider.
type githubApplication struct {
	gh *ghapi.Client

	name    string
	version string
}

func (a *githubApplication) Name() string {
	return a.name
}

func (a *githubApplication) Version() string {
	return a.version
}

func (*githubApplication) Download(_ context.Context, _ string) error {
	return errNotImplemented
}

// An update from the Github provider.
type githubOSUpdate struct {
	gh *ghapi.Client

	version string
}

func (o *githubOSUpdate) Version() string {
	return o.version
}

func (*githubOSUpdate) Download(_ context.Context, _ string) error {
	return errNotImplemented
}

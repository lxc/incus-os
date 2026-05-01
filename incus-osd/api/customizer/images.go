package customizer

import (
	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// ImagesPost represents the data needed for POST /1.0/images.
type ImagesPost struct {
	Architecture apiimages.UpdateFileArchitecture `json:"architecture" yaml:"architecture"`
	Type         string                           `json:"type"         yaml:"type"`
	Channel      string                           `json:"channel"      yaml:"channel"`
	Version      string                           `json:"version"      yaml:"version"`

	Seeds   ImagesPostSeeds `json:"seeds"   yaml:"seeds"`
	Offline bool            `json:"offline" yaml:"offline"`
}

// ImagesPostSeeds represents the seed data included in a ImagesPost request.
type ImagesPostSeeds struct {
	Applications     *apiseed.Applications     `json:"applications"      yaml:"applications"`
	Incus            *apiseed.Incus            `json:"incus"             yaml:"incus"`
	Install          *apiseed.Install          `json:"install"           yaml:"install"`
	MigrationManager *apiseed.MigrationManager `json:"migration-manager" yaml:"migration-manager"` //nolint:tagliatelle
	Network          *apiseed.Network          `json:"network"           yaml:"network"`
	OperationsCenter *apiseed.OperationsCenter `json:"operations-center" yaml:"operations-center"` //nolint:tagliatelle
	Provider         *apiseed.Provider         `json:"provider"          yaml:"provider"`
	Update           *apiseed.Update           `json:"update"            yaml:"update"`
}

// UpdatesPost represents the data needed for POST /1.0/updates.
type UpdatesPost struct {
	UpdateFilter `yaml:",inline"`
}

// UpdateFilter represents filterable parameters for update resources.
type UpdateFilter struct {
	Channel string `json:"channel" yaml:"channel"`
	Version string `json:"version" yaml:"version"`

	Components    []apiimages.UpdateFileComponent    `json:"components"    yaml:"components"`
	Types         []apiimages.UpdateFileType         `json:"types"         yaml:"types"`
	Architectures []apiimages.UpdateFileArchitecture `json:"architectures" yaml:"architectures"`
}

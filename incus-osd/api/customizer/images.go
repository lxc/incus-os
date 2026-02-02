package customizer

import (
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// ImagesPost represents the data needed for POST /1.0/images.
type ImagesPost struct {
	Architecture string          `json:"architecture" yaml:"architecture"`
	Type         string          `json:"type"         yaml:"type"`
	Seeds        ImagesPostSeeds `json:"seeds"        yaml:"seeds"`
	Channel      string          `json:"channel"      yaml:"channel"`
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

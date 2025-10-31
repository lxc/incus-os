package manifests

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
)

// MkosiManifest represents the json manifest produced by mkosi.
type MkosiManifest struct {
	ManifestVersion int                     `json:"manifest_version"`
	Config          MkosiManifestConfig     `json:"config"`
	Packages        []MkosiManifestPackages `json:"packages"`
}

// MkosiManifestConfig represents configuration information of the mkosi manifest.
type MkosiManifestConfig struct {
	Name         string `json:"name"`
	Distribution string `json:"distribution"`
	Architecture string `json:"architecture"`
	Version      string `json:"version"`
	Release      string `json:"release"`
}

// MkosiManifestPackages represents information about a package installed during the mkosi build.
type MkosiManifestPackages struct {
	Type         string `json:"type"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Architecture string `json:"architecture,omitempty"`
	Direct       bool   `json:"direct,omitempty"` // IncusOS addition.
}

// IncusOSManifest is an extension of the mkosi manifest definition.
type IncusOSManifest struct {
	MkosiManifest

	IncusOSCommit string             `json:"incusos_commit"`
	MkosiVersion  string             `json:"mkosi_version"`
	Artifacts     []IncusOSArtifacts `json:"artifacts,omitempty"`
}

// IncusOSArtifacts represents information about an artifact produced outside of the main mkosi build logic.
type IncusOSArtifacts struct {
	Name               string                  `json:"name"`
	Version            string                  `json:"version"`
	Repo               string                  `json:"repo"`
	InstalledArtifacts []string                `json:"installed_artifacts"`
	Packages           []MkosiManifestPackages `json:"packages,omitempty"`
	GoCompiler         string                  `json:"go_compiler,omitempty"`
	GoPackages         []MkosiManifestPackages `json:"go_packages,omitempty"`
	YarnVersion        string                  `json:"yarn_version,omitempty"`
	YarnPackages       []MkosiManifestPackages `json:"yarn_packages,omitempty"`
}

// GenerateManifests creates an IncusOS for each image.
func GenerateManifests(ctx context.Context, root string, manifests map[string]IncusOSManifest) (map[string]IncusOSManifest, error) {
	// Get mkosi version.
	output, err := subprocess.RunCommandContext(ctx, "mkosi", "--version")
	if err != nil {
		return nil, err
	}

	mkosiVersion := strings.TrimSuffix(output, "\n")

	// Get the current IncusOS commit we're building from.
	output, err = subprocess.RunCommandContext(ctx, "git", "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	incusosCommit := strings.TrimSuffix(output, "\n")

	ret := make(map[string]IncusOSManifest)

	// When generating a child image manifest, mkosi annoyingly includes all packages already present
	// in the parent image. That's incorrect, so trim out any packages listed in a child manifest
	// that are present in the base manifest.
	for manifestName := range manifests {
		if manifestName == "base" {
			ret[manifestName] = manifests["base"]
		} else {
			ret[manifestName] = trimChildManifest(manifests["base"], manifests[manifestName])
		}
	}

	// Now, we start mutating the mkosi manifests into IncusOS manifests.

	// Set information about the build environment.
	for manifestName := range ret {
		manifest := ret[manifestName]
		manifest.IncusOSCommit = incusosCommit
		manifest.MkosiVersion = mkosiVersion
		ret[manifestName] = manifest
	}

	// Insert artifact information, if it exists, for each manifest.
	for manifestName := range ret {
		// #nosec G304
		content, err := os.ReadFile(filepath.Join(root, manifestName+".json"))
		if err != nil {
			// No artifacts were injected for this image.
			continue
		}

		manifest := ret[manifestName]

		err = json.Unmarshal(content, &manifest.Artifacts)
		if err != nil {
			return nil, err
		}

		// Special case to inject migration-manager-worker mkosi manifest.
		if manifestName == "migration-manager" {
			// #nosec G304
			content, err := os.ReadFile(filepath.Join(root, "migration-manager/worker/mkosi.output/migration-manager-worker.manifest"))
			if err == nil {
				var m MkosiManifest

				err = json.Unmarshal(content, &m)
				if err != nil {
					return nil, err
				}

				manifest.Artifacts = append(manifest.Artifacts, IncusOSArtifacts{
					Name:               "migration-manager-worker",
					Version:            manifest.Artifacts[0].Version,
					Repo:               manifest.Artifacts[0].Repo,
					InstalledArtifacts: []string{"/usr/share/migration-manager/images/worker-x86_64.img"},
					Packages:           m.Packages,
				})
			}
		}

		ret[manifestName] = manifest
	}

	return ret, nil
}

// DiffManifests compares two manifests for differences between their installed packages.
func DiffManifests(a IncusOSManifest, b IncusOSManifest) apiupdate.ChangelogEntries {
	// Generate diff for mkosi-installed packages.
	added, removed, updated := diffPackages(a.Packages, b.Packages)

	// Generate diff for installed artifacts. For now, don't recurse down into each artifact's dependencies.
	for _, previousArtifact := range a.Artifacts {
		index := slices.IndexFunc(b.Artifacts, func(a IncusOSArtifacts) bool {
			return previousArtifact.Name == a.Name
		})

		if index == -1 {
			// The previous artifact was removed from the current manifest.
			removed = append(removed, MkosiManifestPackages{
				Name:    previousArtifact.Name,
				Version: previousArtifact.Version,
			})
		} else if previousArtifact.Version != b.Artifacts[index].Version {
			// The package artifact was updated.
			updated = append(updated, []MkosiManifestPackages{
				{
					Name:    previousArtifact.Name,
					Version: previousArtifact.Version,
				},
				{
					Name:    b.Artifacts[index].Name,
					Version: b.Artifacts[index].Version,
				},
			})
		}
	}

	for _, currentArtifact := range b.Artifacts {
		index := slices.IndexFunc(a.Artifacts, func(a IncusOSArtifacts) bool {
			return currentArtifact.Name == a.Name
		})

		if index == -1 {
			// The artifact was added in the current manifest.
			added = append(added, MkosiManifestPackages{
				Name:    currentArtifact.Name,
				Version: currentArtifact.Version,
			})
		}
	}

	// Take the computed diffs and generate nice addition/update/removal strings.
	var ret apiupdate.ChangelogEntries

	for _, a := range added {
		ret.Added = append(ret.Added, a.Name+" version "+a.Version)
	}

	for _, u := range updated {
		ret.Updated = append(ret.Updated, u[0].Name+" version "+u[0].Version+" to version "+u[1].Version)
	}

	for _, r := range removed {
		ret.Removed = append(ret.Removed, r.Name+" version "+r.Version)
	}

	return ret
}

// ReadManifests loads an existing manifest created by either mkosi or further processed into an IncusOS
// manifest. We always expect a base manifest to exist and be the first element in the list of manifests.
func ReadManifests(root string, manifests []string) (map[string]IncusOSManifest, error) {
	if len(manifests) == 0 {
		return nil, errors.New("list of manifests cannot be empty")
	}

	if manifests[0] != "base" {
		return nil, errors.New("the first manifest must be 'base'")
	}

	ret := make(map[string]IncusOSManifest)

	for _, manifestName := range manifests {
		var m IncusOSManifest

		file := filepath.Join(root, manifestName+".manifest")

		// If the file doesn't exist with the ".manifest" extension, try ".json".
		_, err := os.Stat(file)
		if err != nil {
			file = filepath.Join(root, manifestName+".json")
		}

		// #nosec G304
		content, err := os.ReadFile(file)
		if err != nil {
			if manifestName == "base" {
				return nil, err
			}

			// If the manifest file doesn't exist, create a minimal one from the base manifest.
			// This happens if mkosi doesn't install any packages during its part of the build.
			m.ManifestVersion = ret["base"].ManifestVersion
			m.Config = ret["base"].Config
		} else {
			err = json.Unmarshal(content, &m)
			if err != nil {
				return nil, err
			}

			// If the child has no version set, use the version from the base manifest.
			if m.Config.Version == "" {
				m.Config.Version = ret["base"].Config.Version
			}

			// Fix weird mkosi reporting of amd64.
			if m.Config.Architecture == "x86-64" {
				m.Config.Architecture = "x86_64"
			}
		}

		ret[manifestName] = m
	}

	return ret, nil
}

// WriteManifests writes each manifest to a json file in the given output root directory.
func WriteManifests(root string, manifests map[string]IncusOSManifest) error {
	// Dump the full manifest file for each image.
	for manifest := range manifests {
		content, err := json.Marshal(manifests[manifest])
		if err != nil {
			return err
		}

		filename := manifest + ".json"
		if manifest == "base" {
			filename = manifests[manifest].Config.Name + "_" + manifests[manifest].Config.Version + ".json"
		}

		err = os.WriteFile(filepath.Join(root, filename), content, 0o644)
		if err != nil {
			return err
		}
	}

	return nil
}

func trimChildManifest(parent IncusOSManifest, child IncusOSManifest) IncusOSManifest {
	ret := IncusOSManifest{
		MkosiManifest: MkosiManifest{
			ManifestVersion: child.ManifestVersion,
			Config:          child.Config,
		},
	}

	// Add the package if it's only present in the child manifest.
	for _, p := range child.Packages {
		if !slices.Contains(parent.Packages, p) {
			ret.Packages = append(ret.Packages, p)
		}
	}

	return ret
}

func diffPackages(previous []MkosiManifestPackages, current []MkosiManifestPackages) ([]MkosiManifestPackages, []MkosiManifestPackages, [][]MkosiManifestPackages) { //nolint:revive
	added := []MkosiManifestPackages{}
	removed := []MkosiManifestPackages{}
	updated := [][]MkosiManifestPackages{}

	// Check for removed or updated packages.
	for _, previousPackage := range previous {
		index := slices.IndexFunc(current, func(p MkosiManifestPackages) bool {
			return previousPackage.Name == p.Name
		})

		if index == -1 {
			// The previous package was removed from the current manifest.
			removed = append(removed, previousPackage)
		} else if previousPackage.Version != current[index].Version {
			// The package version was updated.
			updated = append(updated, []MkosiManifestPackages{previousPackage, current[index]})
		}
	}

	// Check for added packages.
	for _, currentPackage := range current {
		index := slices.IndexFunc(previous, func(p MkosiManifestPackages) bool {
			return currentPackage.Name == p.Name
		})

		if index == -1 {
			// The package was added in the current manifest.
			added = append(added, currentPackage)
		}
	}

	return added, removed, updated
}

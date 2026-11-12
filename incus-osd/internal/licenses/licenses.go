// Package licenses handles the license information shipped in each image.
package licenses

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/lxc/incus-os/incus-osd/api"
)

// Dir is where each image ships its license file.
const Dir = "/usr/share/incus-os/licenses"

// BaseImage is the image holding the common licenses and most packages.
const BaseImage = "base"

// File is the on-disk format of an image's license file.
type File struct {
	Packages map[string]string `yaml:"packages,omitempty"`
	Local    map[string]string `yaml:"local,omitempty"`
	Licenses map[string]string `yaml:"licenses,omitempty"`
}

// Names returns the sorted list of "type:name" entries.
func (f *File) Names() []string {
	names := []string{}

	for licenseType, section := range f.sections() {
		for name := range section {
			names = append(names, string(licenseType)+":"+name)
		}
	}

	slices.Sort(names)

	return names
}

// Get returns the entry matching the provided "type:name" or nil if there is none.
func (f *File) Get(fullName string) *api.License {
	typeName, name, _ := strings.Cut(fullName, ":")
	licenseType := api.LicenseType(typeName)

	content, ok := f.sections()[licenseType][name]
	if !ok {
		return nil
	}

	return &api.License{Name: name, Type: licenseType, Content: content}
}

// sections returns the content of the file indexed by license type.
func (f *File) sections() map[api.LicenseType]map[string]string {
	if f.Packages == nil {
		f.Packages = map[string]string{}
	}

	if f.Local == nil {
		f.Local = map[string]string{}
	}

	if f.Licenses == nil {
		f.Licenses = map[string]string{}
	}

	return map[api.LicenseType]map[string]string{
		api.LicenseTypePackage: f.Packages,
		api.LicenseTypeLocal:   f.Local,
		api.LicenseTypeLicense: f.Licenses,
	}
}

// merge adds all entries from the other file, replacing existing ones.
func (f *File) merge(other *File) {
	sections := f.sections()

	for licenseType, section := range other.sections() {
		maps.Copy(sections[licenseType], section)
	}
}

// subtract removes all entries which are identical in the other file.
func (f *File) subtract(other *File) {
	sections := other.sections()

	for licenseType, section := range f.sections() {
		for name, content := range section {
			if sections[licenseType][name] == content {
				delete(section, name)
			}
		}
	}
}

// readFile parses a single license file.
func readFile(path string) (*File, error) {
	content, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, err
	}

	f := &File{}

	err = yaml.Unmarshal(content, f)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// Load returns the combined license information from all image files in the directory.
func Load(dir string) (*File, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}

	// Load the base image first so that other images can override its entries.
	basePath := filepath.Join(dir, BaseImage+".yaml")

	slices.SortFunc(paths, func(a string, b string) int {
		if a == basePath {
			return -1
		}

		if b == basePath {
			return 1
		}

		return strings.Compare(a, b)
	})

	combined := &File{}

	for _, path := range paths {
		f, err := readFile(path)
		if err != nil {
			return nil, err
		}

		combined.merge(f)
	}

	return combined, nil
}

// readText returns the content of a file as valid UTF-8 text.
func readText(path string) (string, error) {
	content, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return "", err
	}

	return strings.ToValidUTF8(string(content), "�"), nil
}

// readDir returns the text of all files in a directory, keyed by file name.
func readDir(dir string) (map[string]string, error) {
	ret := map[string]string{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ret, nil
		}

		return nil, err
	}

	for _, entry := range entries {
		content, err := readText(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}

		ret[entry.Name()] = content
	}

	return ret, nil
}

// readPackages returns the copyright file of every Debian package found in the root.
func readPackages(root string) (map[string]string, error) {
	ret := map[string]string{}
	docDir := filepath.Join(root, "usr/share/doc")

	entries, err := os.ReadDir(docDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ret, nil
		}

		return nil, err
	}

	for _, entry := range entries {
		pkgDir := filepath.Join(docDir, entry.Name())

		// Some packages share their documentation through a symlink, resolve it within the root.
		if entry.Type()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(pkgDir)
			if err != nil {
				return nil, err
			}

			if filepath.IsAbs(target) {
				pkgDir = filepath.Join(root, target)
			} else {
				pkgDir = filepath.Join(docDir, target)
			}
		}

		content, err := readText(filepath.Join(pkgDir, "copyright"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return nil, err
		}

		ret[entry.Name()] = content
	}

	return ret, nil
}

// Generate writes the license file for an image based on the content of its root file system.
func Generate(image string, root string, localDir string) error {
	var err error

	f := &File{}

	f.Packages, err = readPackages(root)
	if err != nil {
		return err
	}

	f.Local, err = readDir(filepath.Join(localDir, image))
	if err != nil {
		return err
	}

	f.Licenses, err = readDir(filepath.Join(root, "usr/share/common-licenses"))
	if err != nil {
		return err
	}

	targetDir := filepath.Join(root, Dir)

	// Other images only need to record what isn't already covered by the base image.
	if image != BaseImage {
		base, err := readFile(filepath.Join(targetDir, BaseImage+".yaml"))
		if err != nil {
			return err
		}

		f.subtract(base)
	}

	content, err := yaml.Marshal(f)
	if err != nil {
		return err
	}

	err = os.MkdirAll(targetDir, 0o755)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(targetDir, image+".yaml"), content, 0o644) //nolint:gosec
}

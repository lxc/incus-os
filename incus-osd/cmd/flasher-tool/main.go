// Package main is used for the flasher tool.
package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/lxc/incus/v6/shared/ask"
	"github.com/lxc/incus/v6/shared/revert"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
	"github.com/lxc/incus-os/incus-osd/internal/providers"
	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

var version = "dev"

var applicationsSeed *apiseed.Applications

var incusSeed *apiseed.Incus

var installSeed *apiseed.Install

var networkSeed *apiseed.Network

type cmdGlobal struct {
	flagHelp    bool
	flagVersion bool
	flagImage   string
	flagFormat  string
	flagSeedTar string
	flagChannel string
}

func main() {
	// Global flags.
	globalCmd := cmdGlobal{}

	app := &cobra.Command{
		Use:   "flasher-tool",
		Short: "IncusOS installation image flasher tool",
		Long: formatSection("Description",
			"IncusOS installation image flasher tool\n\nThis tool allows for downloading and customizing IncusOS installation images."),
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE:              globalCmd.run,
	}

	app.PersistentFlags().BoolVarP(&globalCmd.flagHelp, "help", "h", false, "Print help command")
	app.PersistentFlags().BoolVarP(&globalCmd.flagVersion, "version", "v", false, "Print binary version")

	app.Flags().StringVarP(&globalCmd.flagImage, "image", "i", "", "Local IncusOS install image to use (disables CDN download)")
	app.Flags().StringVarP(&globalCmd.flagFormat, "format", "f", "", "Image format to download: 'iso' or 'img' (disables format prompt)")
	app.Flags().StringVarP(&globalCmd.flagSeedTar, "seed", "s", "", "Path to install seed tar archive (advanced, disables interactive mode)")
	app.Flags().StringVarP(&globalCmd.flagChannel, "channel", "c", "stable", "Update channel to download from (default: stable)")

	// Help handling.
	app.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})

	// Run the main command and handle errors.
	err := app.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func (c *cmdGlobal) run(_ *cobra.Command, _ []string) error {
	if c.flagVersion {
		_, _ = fmt.Println("flasher-tool version " + version) //nolint:forbidigo

		return nil
	}

	var err error

	ctx := context.Background()

	asker := ask.NewAsker(bufio.NewReader(os.Stdin))

	slog.InfoContext(ctx, "IncusOS flasher tool")

	// Determine what image we should modify.
	imageFilename := c.flagImage
	if imageFilename == "" {
		slog.InfoContext(ctx, "Fetching latest release from the Linux Containers CDN")

		imageFilename, err = downloadCurrentIncusOSRelease(ctx, asker, c.flagFormat, c.flagChannel)
		if err != nil {
			slog.ErrorContext(ctx, err.Error())

			return err
		}
	}

	if c.flagSeedTar == "" {
		// Customize the image.
		slog.InfoContext(ctx, "Ready to begin customizing image '"+imageFilename+"'")

		err = mainMenu(ctx, asker, imageFilename)
		if err != nil {
			slog.ErrorContext(ctx, err.Error())

			return err
		}
	} else {
		// Inject the provided seed data.
		slog.InfoContext(ctx, "Injecting user-provided seed data")

		// #nosec G304
		seedFD, err := os.Open(c.flagSeedTar)
		if err != nil {
			slog.ErrorContext(ctx, err.Error())

			return err
		}

		s, err := seedFD.Stat()
		if err != nil {
			slog.ErrorContext(ctx, err.Error())

			return err
		}

		buf := make([]byte, s.Size())

		numBytes, err := seedFD.Read(buf)
		if err != nil {
			slog.ErrorContext(ctx, err.Error())

			return err
		}

		if int64(numBytes) != s.Size() {
			slog.ErrorContext(ctx, fmt.Sprintf("Only read %d of %d bytes from seed file '%s'", numBytes, s.Size(), c.flagSeedTar))
		}

		err = injectSeedIntoImage(imageFilename, buf)
		if err != nil {
			slog.ErrorContext(ctx, err.Error())

			return err
		}
	}

	slog.InfoContext(ctx, "Done!")

	return nil
}

func mainMenu(ctx context.Context, asker ask.Asker, imageFilename string) error {
	isIMG := strings.HasSuffix(imageFilename, ".img")

	// If configuring an ISO, jump right into install configuration options.
	if !isIMG {
		err := toggleInstallRunningMode(ctx, asker, imageFilename)
		if err != nil {
			return err
		}
	}

	for {
		// Dynamically create menu options depending on what's been selected.
		menuOptions := []string{}
		menuSelectionOptions := []string{}

		if isIMG {
			menuOptions = append(menuOptions, "Toggle default boot mode to install or run")
			menuSelectionOptions = append(menuSelectionOptions, strconv.Itoa(len(menuOptions)))
		}

		menuOptions = append(menuOptions, "Select applications")
		menuSelectionOptions = append(menuSelectionOptions, strconv.Itoa(len(menuOptions)))

		menuOptions = append(menuOptions, "Configure network seed")
		menuSelectionOptions = append(menuSelectionOptions, strconv.Itoa(len(menuOptions)))

		if applicationsSeed != nil && slices.ContainsFunc(applicationsSeed.Applications, func(a apiseed.Application) bool {
			return a.Name == "incus"
		}) {
			menuOptions = append(menuOptions, "Configure Incus seed")
			menuSelectionOptions = append(menuSelectionOptions, strconv.Itoa(len(menuOptions)))
		}

		menuOptions = append(menuOptions, "Write image and exit")
		menuSelectionOptions = append(menuSelectionOptions, strconv.Itoa(len(menuOptions)))

		var menuPrompt strings.Builder

		_, _ = menuPrompt.WriteString("\nCustomization options:\n")
		for i := range menuOptions {
			_, _ = menuPrompt.WriteString(fmt.Sprintf("%s) %s\n", menuSelectionOptions[i], menuOptions[i]))
		}

		_, _ = menuPrompt.WriteString("\nSelection: ")

		// Prompt the user for a selection.
		selection, err := asker.AskChoice(menuPrompt.String(), menuSelectionOptions, strconv.Itoa(len(menuOptions)))
		if err != nil {
			return err
		}

		selectionInt, _ := strconv.Atoi(selection)

		switch menuOptions[selectionInt-1] {
		case "Toggle default boot mode to install or run":
			err := toggleInstallRunningMode(ctx, asker, imageFilename)
			if err != nil {
				return err
			}
		case "Select applications":
			err := selectApplications(asker)
			if err != nil {
				return err
			}
		case "Configure network seed":
			err := configureNetworkSeed(ctx)
			if err != nil {
				return err
			}
		case "Configure Incus seed":
			err := configureIncusSeed(ctx)
			if err != nil {
				return err
			}
		case "Write image and exit":
			return writeImage(asker, imageFilename)
		default:
		}
	}
}

func toggleInstallRunningMode(ctx context.Context, asker ask.Asker, imageFilename string) error {
	if strings.HasSuffix(imageFilename, ".img") {
		defaultInstall, err := asker.AskBool("Default to install mode? [Y/n] ", "y")
		if err != nil {
			return err
		}

		if !defaultInstall {
			// Expand the .img to 50GiB.
			slog.InfoContext(ctx, "Truncating image size to 50GiB")

			err := os.Truncate(imageFilename, 50*1024*1024*1024)
			if err != nil {
				return err
			}

			slog.InfoContext(ctx, "Will default to running IncusOS from boot media")

			installSeed = nil

			return nil
		}

		slog.InfoContext(ctx, "Will default to installing IncusOS from boot media")
	} else {
		slog.InfoContext(ctx, "Configuring default install options")
	}

	targetID, err := asker.AskString("[Optional] Device ID to select install target device: ", "", func(_ string) error { return nil })
	if err != nil {
		return err
	}

	forceInstall, err := asker.AskBool("Force install even if partitions exist on the target device? (WARNING: THIS CAN CAUSE DATA LOSS!) [y/N] ", "n")
	if err != nil {
		return err
	}

	forceReboot, err := asker.AskBool("Force reboot after install without waiting for removal of install media? [y/N] ", "n")
	if err != nil {
		return err
	}

	var target *apiseed.InstallTarget
	if targetID != "" {
		target = &apiseed.InstallTarget{
			ID: targetID,
		}
	}

	installSeed = &apiseed.Install{
		ForceInstall: forceInstall,
		ForceReboot:  forceReboot,
		Target:       target,
	}

	return nil
}

func selectApplications(asker ask.Asker) error {
	installIncus, err := asker.AskBool("Install Incus? [Y/n] ", "y")
	if err != nil {
		return err
	}

	applicationsSeed = &apiseed.Applications{}

	if installIncus {
		applicationsSeed.Applications = append(applicationsSeed.Applications, apiseed.Application{
			Name: "incus",
		})
	}

	return nil
}

func configureNetworkSeed(ctx context.Context) error {
	var err error

	existingContents := []byte("# Provide network seed in yaml format")

	if networkSeed != nil {
		existingContents, err = yaml.Marshal(networkSeed)
		if err != nil {
			return err
		}
	}

	// Launch an editor to allow user to provide a network seed.
	newContents, err := textEditor(ctx, existingContents)
	if err != nil {
		slog.ErrorContext(ctx, err.Error())

		return nil
	}

	var newSeed apiseed.Network

	err = yaml.Unmarshal(newContents, &newSeed)
	if err != nil {
		slog.ErrorContext(ctx, err.Error())

		return nil
	}

	// Validate the network seed.
	if seed.NetworkConfigHasEmptyDevices(newSeed.SystemNetworkConfig) {
		slog.ErrorContext(ctx, "provided network seed has no interfaces, bonds, or vlans defined")

		return nil
	}

	err = systemd.ValidateNetworkConfiguration(&newSeed.SystemNetworkConfig, false)
	if err != nil {
		slog.ErrorContext(ctx, err.Error())

		return nil
	}

	// Save the validated network seed.
	networkSeed = &newSeed

	return nil
}

func configureIncusSeed(ctx context.Context) error {
	var err error

	existingContents := []byte("# Provide Incus seed in yaml format")

	if incusSeed != nil {
		existingContents, err = yaml.Marshal(incusSeed)
		if err != nil {
			return err
		}
	}

	// Launch an editor to allow user to provide an Incus seed.
	newContents, err := textEditor(ctx, existingContents)
	if err != nil {
		slog.ErrorContext(ctx, err.Error())

		return nil
	}

	var newSeed apiseed.Incus

	err = yaml.Unmarshal(newContents, &newSeed)
	if err != nil {
		slog.ErrorContext(ctx, err.Error())

		return nil
	}

	// Save the Incus seed.
	incusSeed = &newSeed

	return nil
}

func writeImage(asker ask.Asker, sourceImage string) error {
	targetImage, err := asker.AskString("Filename to write image to ["+sourceImage+"]: ", sourceImage, nil)
	if err != nil {
		return err
	}

	// Copy the image, if needed.
	if targetImage != sourceImage {
		// #nosec G304
		src, err := os.Open(sourceImage)
		if err != nil {
			return err
		}
		defer src.Close()

		// #nosec G304
		tgt, err := os.Create(targetImage)
		if err != nil {
			return nil
		}
		defer tgt.Close()

		_, err = io.Copy(tgt, src)
		if err != nil {
			return err
		}
	}

	archiveContents := [][]string{}

	// Create applications yaml contents.
	if applicationsSeed != nil {
		yamlContents, err := yaml.Marshal(applicationsSeed)
		if err != nil {
			return err
		}

		archiveContents = append(archiveContents, []string{"applications.yaml", string(yamlContents)})
	}

	// Create incus yaml contents.
	if incusSeed != nil {
		yamlContents, err := yaml.Marshal(incusSeed)
		if err != nil {
			return err
		}

		archiveContents = append(archiveContents, []string{"incus.yaml", string(yamlContents)})
	}

	// Create install yaml contents.
	if installSeed != nil {
		yamlContents, err := yaml.Marshal(installSeed)
		if err != nil {
			return err
		}

		archiveContents = append(archiveContents, []string{"install.yaml", string(yamlContents)})
	}

	// Create network yaml contents.
	if networkSeed != nil {
		yamlContents, err := yaml.Marshal(networkSeed)
		if err != nil {
			return err
		}

		archiveContents = append(archiveContents, []string{"network.yaml", string(yamlContents)})
	}

	// Create the tar archive.
	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	for _, file := range archiveContents {
		hdr := &tar.Header{
			Name: file[0],
			Mode: 0o600,
			Size: int64(len(file[1])),
		}

		err := tw.WriteHeader(hdr)
		if err != nil {
			return err
		}

		_, err = tw.Write([]byte(file[1]))
		if err != nil {
			return err
		}
	}

	err = tw.Close()
	if err != nil {
		return err
	}

	return injectSeedIntoImage(targetImage, buf.Bytes())
}

func injectSeedIntoImage(imageFilename string, data []byte) error {
	// Copy the seed data into the image.
	// #nosec G304
	tgt, err := os.OpenFile(imageFilename, os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer tgt.Close()

	numBytes, err := tgt.WriteAt(data, 2148532224)
	if err != nil {
		return err
	}

	if numBytes != len(data) {
		return fmt.Errorf("failed to write seed tar archive into image: copied %d of %d bytes", numBytes, len(data))
	}

	return nil
}

func downloadCurrentIncusOSRelease(ctx context.Context, asker ask.Asker, imageFormat string, channel string) (string, error) {
	s := state.State{}
	s.System.Provider.Config.Name = "images"
	s.System.Update.Config.Channel = channel

	provider, err := providers.Load(ctx, &s)
	if err != nil {
		return "", err
	}

	if imageFormat == "" {
		imageFormat, err = asker.AskChoice("Image format (iso or img): ", []string{"iso", "img"}, "iso")
		if err != nil {
			return "", err
		}
	}

	// Get the latest release.
	release, err := provider.GetOSUpdate(ctx)
	if err != nil {
		return "", err
	}

	slog.InfoContext(ctx, "Downloading and decompressing IncusOS image ("+imageFormat+") version "+release.Version()+" from Linux Containers CDN")

	// Download and decompress the image.
	return release.DownloadImage(ctx, imageFormat, ".", nil)
}

// Spawn the editor with a temporary YAML file for editing configs.
// Stolen from incus/cmd/incus/utils.go.
func textEditor(ctx context.Context, inContent []byte) ([]byte, error) {
	var f *os.File

	var err error

	var path string

	// Detect the text editor to use
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			for _, p := range []string{"editor", "vi", "emacs", "nano"} {
				_, err := exec.LookPath(p)
				if err == nil {
					editor = p

					break
				}
			}

			if editor == "" {
				return []byte{}, errors.New("no text editor found, please set the EDITOR environment variable")
			}
		}
	}

	// If provided input, create a new file
	f, err = os.CreateTemp("", "incus_editor_")
	if err != nil {
		return []byte{}, err
	}

	reverter := revert.New()
	defer reverter.Fail()

	reverter.Add(func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	})

	err = os.Chmod(f.Name(), 0o600)
	if err != nil {
		return []byte{}, err
	}

	_, err = f.Write(inContent)
	if err != nil {
		return []byte{}, err
	}

	err = f.Close()
	if err != nil {
		return []byte{}, err
	}

	path = f.Name() + ".yaml"

	err = os.Rename(f.Name(), path)
	if err != nil {
		return []byte{}, err
	}

	reverter.Success()
	reverter.Add(func() { _ = os.Remove(path) })

	cmdParts := strings.Fields(editor)
	// #nosec G204
	cmd := exec.CommandContext(ctx, cmdParts[0], append(cmdParts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return []byte{}, err
	}

	// #nosec G304
	content, err := os.ReadFile(path)
	if err != nil {
		return []byte{}, err
	}

	return content, nil
}

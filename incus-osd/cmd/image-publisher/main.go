// Package main is used for the image publisher.
package main

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

type cmdGlobal struct {
	flagHelp bool
}

func main() {
	app := &cobra.Command{}
	app.Use = "image-publisher"
	app.Short = "Maintains an Incus OS update server"
	app.Long = `Description:
  Maintain an Incus OS update server

  This tool handles publishing, promotion and cleanup of Incus OS updates.
`
	app.SilenceUsage = true
	app.CompletionOptions = cobra.CompletionOptions{DisableDefaultCmd: true}

	// Global flags.
	globalCmd := cmdGlobal{}
	app.PersistentFlags().BoolVarP(&globalCmd.flagHelp, "help", "h", false, "Print help")

	// Help handling.
	app.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})

	// promote sub-command.
	promoteCmd := cmdPromote{global: &globalCmd}
	app.AddCommand(promoteCmd.command())

	// prune sub-command.
	pruneCmd := cmdPrune{global: &globalCmd}
	app.AddCommand(pruneCmd.command())

	// sync sub-command.
	syncCmd := cmdSync{global: &globalCmd}
	app.AddCommand(syncCmd.command())

	// Run the main command and handle errors.
	err := app.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// CheckArgs validates the number of arguments passed to the function and shows the help if incorrect.
func (*cmdGlobal) CheckArgs(cmd *cobra.Command, args []string, minArgs int, maxArgs int) (bool, error) {
	if len(args) < minArgs || (maxArgs != -1 && len(args) > maxArgs) {
		_ = cmd.Help()

		if len(args) == 0 {
			return true, nil
		}

		return true, errors.New("invalid number of arguments")
	}

	return false, nil
}

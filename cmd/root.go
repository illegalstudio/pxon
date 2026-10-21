package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"pxon/internal/config"
	"pxon/internal/ui"
	"pxon/internal/version"
)

var (
	cfg        *config.Config
	cfgErr     error
	outputJSON bool
)

var rootCmd = &cobra.Command{
	Use:           "pxon",
	Short:         "CLI for creating and managing LXC containers on Proxmox VE",
	Version:       version.String(),
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() error {
	command, err := rootCmd.ExecuteC()
	if err != nil && jsonEnabled() {
		var reported reportedError
		if !errors.As(err, &reported) {
			if writeErr := ui.WriteJSON(rootCmd.OutOrStdout(), map[string]string{"error": err.Error()}); writeErr != nil {
				return errors.Join(err, writeErr)
			}
		}
	}
	if err == nil {
		maybeNoticeStaleSkill(command)
	}
	return err
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Return JSON output")
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

func initConfig() {
	cfg, cfgErr = config.Load()
}

func currentConfig() (*config.Config, error) {
	if cfgErr != nil {
		return nil, cfgErr
	}

	if cfg == nil {
		return nil, config.ErrConfigNotLoaded
	}

	return cfg, nil
}

func jsonEnabled() bool {
	return outputJSON
}

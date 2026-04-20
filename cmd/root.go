package cmd

import (
	"github.com/spf13/cobra"

	"pxon/internal/config"
)

var (
	cfg        *config.Config
	cfgErr     error
	outputJSON bool
)

var rootCmd = &cobra.Command{
	Use:           "pxon",
	Short:         "CLI per creare e gestire container LXC su Proxmox VE",
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Restituisce output JSON")
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

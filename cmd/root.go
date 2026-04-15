package cmd

import (
	"github.com/spf13/cobra"

	"pxon/internal/config"
)

var (
	cfg    *config.Config
	cfgErr error
)

var rootCmd = &cobra.Command{
	Use:           "pxon",
	Short:         "CLI per gestire macchine virtuali su Proxmox VE",
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
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

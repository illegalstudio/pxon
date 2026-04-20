package cmd

import (
	"github.com/spf13/cobra"

	"pxon/internal/proxmox"
	"pxon/internal/ui"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Elenca i container LXC gestiti da pxon",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := currentConfig()
		if err != nil {
			return err
		}

		client, err := proxmox.NewClient(cfg)
		if err != nil {
			return err
		}

		containers, err := client.ManagedContainers()
		if err != nil {
			return err
		}

		if jsonEnabled() {
			return ui.WriteJSON(cmd.OutOrStdout(), map[string]any{"data": containers})
		}

		ui.RenderManagedContainers(cmd.OutOrStdout(), containers)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

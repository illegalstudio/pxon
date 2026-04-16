package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"pxon/internal/proxmox"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Elenca i container LXC disponibili",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := currentConfig()
		if err != nil {
			return err
		}

		client, err := proxmox.NewClient(cfg)
		if err != nil {
			return err
		}

		data, err := client.ListContainers()
		if err != nil {
			return err
		}

		var pretty bytes.Buffer
		if err := json.Indent(&pretty, data, "", "  "); err != nil {
			return fmt.Errorf("invalid JSON response from Proxmox: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), pretty.String())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

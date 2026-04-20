package cmd

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"pxon/internal/config"
	"pxon/internal/network"
	"pxon/internal/proxmox"
	"pxon/internal/sshkey"
	"pxon/internal/ui"
	"pxon/internal/wizard"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configura i default di pxon con un wizard interattivo",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := runConfigWizard(cmd)
		if errors.Is(err, wizard.ErrCancelled) {
			fmt.Fprintln(cmd.ErrOrStderr(), "Annullato.")
			return nil
		}
		return err
	},
}

func runConfigWizard(cmd *cobra.Command) error {
	currentCfg, err := currentConfig()
	if err != nil {
		return err
	}

	client, err := proxmox.NewClient(currentCfg)
	if err != nil {
		return err
	}

	node, err := client.DefaultNode()
	if err != nil {
		return err
	}

	var writer io.Writer = cmd.OutOrStdout()
	if jsonEnabled() {
		writer = cmd.ErrOrStderr()
	}

	storages, err := client.ListStorages(node)
	if err != nil {
		return err
	}
	if len(storages) == 0 {
		return fmt.Errorf("no Proxmox storage available for LXC rootfs on node %q", node)
	}

	if len(storages) == 1 {
		currentCfg.DefaultStorage = storages[0].Storage
		fmt.Fprintf(writer, "Storage disponibile unico rilevato: %s\n", currentCfg.DefaultStorage)
	} else {
		options := make([]string, 0, len(storages))
		defaultIndex := 0
		for i, s := range storages {
			options = append(options, s.Storage)
			if s.Storage == currentCfg.DefaultStorage {
				defaultIndex = i
			}
		}
		currentCfg.DefaultStorage, err = wizard.Select("Seleziona lo storage di default per i container:", options, defaultIndex)
		if err != nil {
			return err
		}
	}

	templates, err := client.ListTemplates(node)
	if err != nil {
		return err
	}
	if len(templates) == 0 {
		return fmt.Errorf("no LXC template available on node %q; download one in Proxmox before running pxon config", node)
	}

	if len(templates) == 1 {
		currentCfg.DefaultImage = templates[0].VolID
		fmt.Fprintf(writer, "Template disponibile unico rilevato: %s\n", currentCfg.DefaultImage)
	} else {
		options := make([]string, 0, len(templates))
		defaultIndex := 0
		for i, t := range templates {
			options = append(options, t.VolID)
			if t.VolID == currentCfg.DefaultImage {
				defaultIndex = i
			}
		}
		currentCfg.DefaultImage, err = wizard.Select("Seleziona il template LXC di default:", options, defaultIndex)
		if err != nil {
			return err
		}
	}

	currentCfg.DefaultDiskSize, err = wizard.Input("Disk size di default", currentCfg.DefaultDiskSize, true)
	if err != nil {
		return err
	}

	currentCfg.DefaultPassword, err = wizard.Input("Password di default", currentCfg.DefaultPassword, true)
	if err != nil {
		return err
	}

	currentCfg.DefaultSSHPublicKeyPath, err = selectSSHPublicKeyPath(currentCfg.DefaultSSHPublicKeyPath)
	if err != nil {
		return err
	}

	mode, err := wizard.Select(
		"Modalità rete di default:",
		[]string{"dhcp", "pool"},
		defaultNetworkModeIndex(currentCfg.Network.Mode),
	)
	if err != nil {
		return err
	}
	currentCfg.Network.Mode = mode
	currentCfg.DefaultNet0 = ""

	bridges, err := client.ListBridges(node)
	if err != nil {
		return err
	}

	if len(bridges) == 1 {
		currentCfg.Network.Bridge = bridges[0]
		fmt.Fprintf(writer, "Bridge disponibile unico rilevato: %s\n", currentCfg.Network.Bridge)
	} else if len(bridges) > 1 {
		defaultIndex := 0
		for i, b := range bridges {
			if b == currentCfg.Network.Bridge {
				defaultIndex = i
				break
			}
		}
		currentCfg.Network.Bridge, err = wizard.Select("Seleziona il bridge di default:", bridges, defaultIndex)
		if err != nil {
			return err
		}
	} else {
		currentCfg.Network.Bridge, err = wizard.Input("Bridge di default", currentCfg.Network.Bridge, true)
		if err != nil {
			return err
		}
	}

	if mode == "pool" {
		currentCfg.Network.Gateway, err = wizard.Input("Gateway di default", currentCfg.Network.Gateway, true)
		if err != nil {
			return err
		}

		netmaskDefault := currentCfg.Network.Netmask
		if netmaskDefault == "" && currentCfg.Network.CIDR > 0 {
			netmaskDefault = strconv.Itoa(currentCfg.Network.CIDR)
		}

		netmaskInput, err := wizard.Input("Maschera di rete", netmaskDefault, true)
		if err != nil {
			return err
		}
		if _, err := network.PrefixFromNetmask(netmaskInput); err != nil {
			return err
		}
		currentCfg.Network.Netmask = netmaskInput
		currentCfg.Network.CIDR = 0

		currentCfg.Network.RangeStart, err = wizard.Input("IP iniziale del pool", currentCfg.Network.RangeStart, true)
		if err != nil {
			return err
		}

		currentCfg.Network.RangeEnd, err = wizard.Input("IP finale del pool", currentCfg.Network.RangeEnd, true)
		if err != nil {
			return err
		}
	} else {
		currentCfg.Network.Gateway = ""
		currentCfg.Network.Netmask = ""
		currentCfg.Network.CIDR = 0
		currentCfg.Network.RangeStart = ""
		currentCfg.Network.RangeEnd = ""
	}

	if err := config.Save(currentCfg); err != nil {
		return err
	}

	cfg = currentCfg

	summary := ui.NewConfigSummary(currentCfg, config.ConfigFilePath())
	if jsonEnabled() {
		return ui.WriteJSON(cmd.OutOrStdout(), summary)
	}

	ui.RenderConfigSummary(writer, summary)
	return nil
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func defaultNetworkModeIndex(mode string) int {
	if strings.ToLower(strings.TrimSpace(mode)) == "pool" {
		return 1
	}
	return 0
}

func selectSSHPublicKeyPath(current string) (string, error) {
	keys, err := sshkey.ListPublicKeys()
	if err != nil {
		return "", err
	}

	current = strings.TrimSpace(current)

	if len(keys) == 0 {
		return wizard.Input("Chiave SSH pubblica di default (opzionale)", current, false)
	}

	const optNone = "Nessuna"
	const optManual = "Percorso manuale..."

	options := make([]string, 0, len(keys)+2)
	options = append(options, optNone)
	options = append(options, keys...)
	options = append(options, optManual)

	defaultIndex := 0
	if current != "" {
		for i, key := range keys {
			if key == current {
				defaultIndex = i + 1
				break
			}
		}
		if defaultIndex == 0 {
			defaultIndex = len(options) - 1
		}
	} else if len(keys) == 1 {
		defaultIndex = 1
	}

	selection, err := wizard.Select("Chiave SSH pubblica di default:", options, defaultIndex)
	if err != nil {
		return "", err
	}

	switch selection {
	case optNone:
		return "", nil
	case optManual:
		return wizard.Input("Percorso chiave SSH pubblica", current, false)
	default:
		return selection, nil
	}
}

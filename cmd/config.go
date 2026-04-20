package cmd

import (
	"bufio"
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
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configura i default di pxon con un wizard interattivo",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		reader := bufio.NewReader(cmd.InOrStdin())
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
			for index, storage := range storages {
				options = append(options, storage.Storage)
				if storage.Storage == currentCfg.DefaultStorage {
					defaultIndex = index
				}
			}

			selection, err := promptSelection(reader, writer, "Seleziona lo storage di default per i container:", options, defaultIndex)
			if err != nil {
				return err
			}
			currentCfg.DefaultStorage = selection
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
			for index, template := range templates {
				options = append(options, template.VolID)
				if template.VolID == currentCfg.DefaultImage {
					defaultIndex = index
				}
			}

			selection, err := promptSelection(reader, writer, "Seleziona il template LXC di default:", options, defaultIndex)
			if err != nil {
				return err
			}
			currentCfg.DefaultImage = selection
		}

		diskSize, err := promptValue(reader, writer, "Disk size di default", currentCfg.DefaultDiskSize, true)
		if err != nil {
			return err
		}
		currentCfg.DefaultDiskSize = diskSize

		password, err := promptValue(reader, writer, "Password di default", currentCfg.DefaultPassword, true)
		if err != nil {
			return err
		}
		currentCfg.DefaultPassword = password

		sshKeyPath, err := promptSSHPublicKeyPath(reader, writer, currentCfg.DefaultSSHPublicKeyPath)
		if err != nil {
			return err
		}
		currentCfg.DefaultSSHPublicKeyPath = sshKeyPath

		mode, err := promptSelection(
			reader,
			writer,
			"Modalita' rete di default:",
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
			for index, bridge := range bridges {
				if bridge == currentCfg.Network.Bridge {
					defaultIndex = index
					break
				}
			}

			bridge, err := promptSelection(reader, writer, "Seleziona il bridge di default:", bridges, defaultIndex)
			if err != nil {
				return err
			}
			currentCfg.Network.Bridge = bridge
		} else {
			bridge, err := promptValue(reader, writer, "Bridge di default", currentCfg.Network.Bridge, true)
			if err != nil {
				return err
			}
			currentCfg.Network.Bridge = bridge
		}

		if mode == "pool" {
			gateway, err := promptValue(reader, writer, "Gateway di default", currentCfg.Network.Gateway, true)
			if err != nil {
				return err
			}
			currentCfg.Network.Gateway = gateway

			netmaskDefault := currentCfg.Network.Netmask
			if netmaskDefault == "" && currentCfg.Network.CIDR > 0 {
				netmaskDefault = strconv.Itoa(currentCfg.Network.CIDR)
			}

			netmaskInput, err := promptValue(reader, writer, "Maschera di rete", netmaskDefault, true)
			if err != nil {
				return err
			}

			if _, err := network.PrefixFromNetmask(netmaskInput); err != nil {
				return err
			}
			currentCfg.Network.Netmask = netmaskInput
			currentCfg.Network.CIDR = 0

			rangeStart, err := promptValue(reader, writer, "IP iniziale del pool", currentCfg.Network.RangeStart, true)
			if err != nil {
				return err
			}
			currentCfg.Network.RangeStart = rangeStart

			rangeEnd, err := promptValue(reader, writer, "IP finale del pool", currentCfg.Network.RangeEnd, true)
			if err != nil {
				return err
			}
			currentCfg.Network.RangeEnd = rangeEnd
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
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func promptSelection(reader *bufio.Reader, writer interface{ Write([]byte) (int, error) }, label string, options []string, defaultIndex int) (string, error) {
	fmt.Fprintln(writer, label)
	for index, option := range options {
		fmt.Fprintf(writer, "  %d) %s\n", index+1, option)
	}

	for {
		fmt.Fprintf(writer, "Scelta [%d]: ", defaultIndex+1)
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read selection: %w", err)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			return options[defaultIndex], nil
		}

		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(options) {
			fmt.Fprintln(writer, "Valore non valido, inserisci il numero di una delle opzioni.")
			continue
		}

		return options[choice-1], nil
	}
}

func defaultNetworkModeIndex(mode string) int {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "pool":
		return 1
	default:
		return 0
	}
}

func promptValue(reader *bufio.Reader, writer interface{ Write([]byte) (int, error) }, label, defaultValue string, required bool) (string, error) {
	for {
		if defaultValue != "" {
			fmt.Fprintf(writer, "%s [%s]: ", label, defaultValue)
		} else {
			fmt.Fprintf(writer, "%s: ", label)
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read input: %w", err)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			if defaultValue != "" {
				return defaultValue, nil
			}

			if required {
				fmt.Fprintln(writer, "Valore richiesto.")
				continue
			}
		}

		return input, nil
	}
}

func promptSSHPublicKeyPath(reader *bufio.Reader, writer interface{ Write([]byte) (int, error) }, current string) (string, error) {
	keys, err := sshkey.ListPublicKeys()
	if err != nil {
		return "", err
	}

	current = strings.TrimSpace(current)

	if len(keys) == 0 {
		return promptValue(reader, writer, "Chiave SSH pubblica di default (opzionale)", current, false)
	}

	fmt.Fprintln(writer, "Chiave SSH pubblica di default:")
	for index, key := range keys {
		fmt.Fprintf(writer, "  %d) %s\n", index+1, key)
	}
	fmt.Fprintln(writer, "  m) inserisci un percorso manuale")
	fmt.Fprintln(writer, "  0) nessuna")

	defaultPrompt := "0"
	if current != "" {
		for index, key := range keys {
			if key == current {
				defaultPrompt = strconv.Itoa(index + 1)
				break
			}
		}
		if defaultPrompt == "0" {
			defaultPrompt = "m"
		}
	} else if len(keys) == 1 {
		defaultPrompt = "1"
	}

	for {
		fmt.Fprintf(writer, "Scelta [%s]: ", defaultPrompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read selection: %w", err)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			input = defaultPrompt
		}

		switch input {
		case "0":
			return "", nil
		case "m", "M":
			return promptValue(reader, writer, "Percorso chiave SSH pubblica", current, false)
		default:
			choice, err := strconv.Atoi(input)
			if err != nil || choice < 1 || choice > len(keys) {
				fmt.Fprintln(writer, "Valore non valido, inserisci un numero, m oppure 0.")
				continue
			}

			return keys[choice-1], nil
		}
	}
}

package cmd

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"pxon/internal/proxmox"
	"pxon/internal/ui"
)

type imageCreateResult struct {
	Image  proxmox.Image       `json:"image"`
	Source proxmox.Container   `json:"source"`
	Stage  string              `json:"stage"`
	Task   *proxmox.TaskStatus `json:"task,omitempty"`
	UPID   string              `json:"upid,omitempty"`
	Error  string              `json:"error,omitempty"`
}

var imagesCmd = &cobra.Command{
	Use:     "images",
	Aliases: []string{"image"},
	Short:   "List reusable images and create templates from configured containers",
}

var imagesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List LXC archives and pxon-managed container templates",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, node, err := imageClientAndNode(cmd)
		if err != nil {
			return err
		}
		images, err := client.ListImages(node)
		if err != nil {
			return err
		}
		if jsonEnabled() {
			return ui.WriteJSON(cmd.OutOrStdout(), map[string]any{"data": images})
		}
		ui.RenderImages(cmd.OutOrStdout(), images)
		return nil
	},
}

var imagesCreateCmd = &cobra.Command{
	Use:   "create <source-name|vmid> <image-name>",
	Short: "Create a full copy of a managed container as a reusable template",
	Args:  cobra.ExactArgs(2),
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
		if node, _ := cmd.Flags().GetString("node"); strings.TrimSpace(node) != "" {
			filtered := make([]proxmox.Container, 0)
			for _, container := range containers {
				if container.Node == node {
					filtered = append(filtered, container)
				}
			}
			containers = filtered
		}
		source, err := findContainer(containers, args[0])
		if err != nil {
			return err
		}
		snapshot, _ := cmd.Flags().GetString("snapshot")
		if source.Template == 1 {
			return fmt.Errorf("source is already a template; use pxon create --image lxc:%d", source.VMID)
		}
		if source.Status != "stopped" && strings.TrimSpace(snapshot) == "" {
			return fmt.Errorf("source container %d is %s; stop it yourself or pass --snapshot <existing-snapshot> to copy it without stopping it", source.VMID, source.Status)
		}
		vmid, _ := cmd.Flags().GetInt("vmid")
		if vmid == 0 {
			vmid, err = client.NextID()
			if err != nil {
				return err
			}
		}
		storage, _ := cmd.Flags().GetString("storage")
		request := proxmox.CloneContainerRequest{Node: source.Node, SourceVMID: source.VMID, VMID: vmid, Hostname: strings.TrimSpace(args[1]), Storage: strings.TrimSpace(storage), Snapshot: strings.TrimSpace(snapshot)}
		if err := request.Validate(); err != nil {
			return err
		}
		result := imageCreateResult{
			Image:  proxmox.Image{ID: fmt.Sprintf("lxc:%d", vmid), Kind: proxmox.ImageTemplate, Name: request.Hostname, Node: source.Node, VMID: vmid},
			Source: source,
			Stage:  "clone",
		}
		finish := func(operationErr error) error {
			if operationErr != nil {
				result.Error = fmt.Sprintf("image %s on %s, stage %s: %v; inspect VMID %d before retrying, the source was not stopped or converted", result.Image.ID, source.Node, result.Stage, operationErr, vmid)
				operationErr = fmt.Errorf("%s", result.Error)
			}
			if jsonEnabled() {
				return writeJSONResult(cmd, result, operationErr)
			}
			if operationErr == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Created image %s (%s) on %s from %s (VMID %d).\n", result.Image.Name, result.Image.ID, source.Node, source.Name, source.VMID)
			}
			return operationErr
		}
		result.UPID, err = client.StartCloneContainer(request)
		if err != nil {
			return finish(err)
		}
		if !jsonEnabled() {
			fmt.Fprintf(cmd.ErrOrStderr(), "Copying %s (VMID %d) as image %s, task %s...\n", source.Name, source.VMID, result.Image.ID, result.UPID)
		}
		result.Task, err = waitForImageTask(client, source.Node, result.UPID)
		if err != nil {
			return finish(err)
		}
		result.Stage = "configure"
		cloneConfig, err := client.ContainerConfig(source.Node, vmid)
		if err != nil {
			return finish(err)
		}
		values := isolatedNetworkConfig(cloneConfig, false)
		values.Set("tags", proxmox.JoinManagedTags([]string{"pxon-image"}))
		if err := client.UpdateContainer(source.Node, vmid, values); err != nil {
			return finish(err)
		}
		result.Stage = "template"
		if err := client.ConvertToTemplate(source.Node, vmid); err != nil {
			return finish(err)
		}
		result.Stage = "completed"
		return finish(nil)
	},
}

func init() {
	rootCmd.AddCommand(imagesCmd)
	imagesCmd.AddCommand(imagesListCmd, imagesCreateCmd)
	imagesListCmd.Flags().String("node", "", "Proxmox node; when omitted, use the default node")
	imagesCreateCmd.Flags().String("node", "", "Restrict the source lookup to this node; the image stays on the source node")
	imagesCreateCmd.Flags().Int("vmid", 0, "New VMID for the image; when omitted, request the next available ID")
	imagesCreateCmd.Flags().String("storage", "", "Storage for the full copy; when omitted, use the source storage")
	imagesCreateCmd.Flags().String("snapshot", "", "Existing source snapshot to copy; required when the source is running")
}

func imageClientAndNode(cmd *cobra.Command) (*proxmox.Client, string, error) {
	cfg, err := currentConfig()
	if err != nil {
		return nil, "", err
	}
	client, err := proxmox.NewClient(cfg)
	if err != nil {
		return nil, "", err
	}
	node, _ := cmd.Flags().GetString("node")
	node = strings.TrimSpace(node)
	if node == "" {
		node, err = client.DefaultNode()
	}
	return client, node, err
}

func waitForImageTask(client *proxmox.Client, node, upid string) (*proxmox.TaskStatus, error) {
	status, err := client.WaitForTask(node, upid, 10*time.Minute)
	if err != nil {
		return nil, err
	}
	if status.ExitStatus != "OK" {
		return status, fmt.Errorf("Proxmox task %s failed: %s", upid, status.ExitStatus)
	}
	return status, nil
}

// Drop inherited interfaces before assigning a fresh network or making a template.
func isolatedNetworkConfig(cfg *proxmox.LXCConfig, keepNet0 bool) url.Values {
	values := url.Values{"onboot": {"0"}}
	var keys []string
	for _, key := range cfg.NetworkKeys {
		if !keepNet0 || key != "net0" {
			keys = append(keys, key)
		}
	}
	if len(keys) > 0 {
		values.Set("delete", strings.Join(keys, ","))
	}
	return values
}

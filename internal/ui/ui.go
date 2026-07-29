package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"

	"pxon/internal/config"
	"pxon/internal/proxmox"
	"pxon/internal/theme"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.TitleColor)
	labelStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.LabelColor)
	valueStyle = lipgloss.NewStyle().Foreground(theme.TextColor)
	mutedStyle = lipgloss.NewStyle().Foreground(theme.MutedColor)
	okStyle    = lipgloss.NewStyle().Bold(true).Foreground(theme.SuccessColor)
)

type ConfigSummary struct {
	Path                    string `json:"path"`
	DefaultStorage          string `json:"default_storage"`
	DefaultImage            string `json:"default_image"`
	DefaultDiskSize         string `json:"default_disk_size"`
	DefaultSSHPublicKeyPath string `json:"default_ssh_public_key_path,omitempty"`
	NetworkMode             string `json:"network_mode"`
	Bridge                  string `json:"bridge,omitempty"`
	Gateway                 string `json:"gateway,omitempty"`
	Netmask                 string `json:"netmask,omitempty"`
	RangeStart              string `json:"range_start,omitempty"`
	RangeEnd                string `json:"range_end,omitempty"`
}

type CreateSummary struct {
	Hostname   string `json:"hostname"`
	VMID       int    `json:"vmid"`
	Node       string `json:"node"`
	Status     string `json:"status"`
	Template   string `json:"template"`
	RootFS     string `json:"rootfs"`
	Net0       string `json:"net0"`
	SSHKeyPath string `json:"ssh_key_path,omitempty"`
	UPID       string `json:"upid"`
}

func WriteJSON(writer io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(writer, string(data))
	return err
}

func RenderManagedContainers(writer io.Writer, containers []proxmox.Container) {
	if len(containers) == 0 {
		fmt.Fprintln(writer, titleStyle.Render("Managed Containers"))
		fmt.Fprintln(writer, mutedStyle.Render("No pxon-managed containers found."))
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(writer)
	t.AppendHeader(table.Row{"VMID", "Name", "Status", "IP", "Node", "Uptime", "Mem", "Disk"})
	for _, container := range containers {
		ip := container.IP
		if ip == "" {
			ip = "-"
		}
		t.AppendRow(table.Row{
			container.VMID,
			container.Name,
			container.Status,
			ip,
			container.Node,
			formatDuration(container.Uptime),
			formatBytes(container.Mem),
			formatBytes(container.Disk),
		})
	}

	t.SetStyle(table.StyleRounded)
	fmt.Fprintln(writer, titleStyle.Render("Managed Containers"))
	t.Render()
}

func RenderCreateSummary(writer io.Writer, summary CreateSummary) {
	fmt.Fprintln(writer, okStyle.Render("Container Created"))
	renderKV(writer, "Hostname", summary.Hostname)
	renderKV(writer, "VMID", fmt.Sprintf("%d", summary.VMID))
	renderKV(writer, "Node", summary.Node)
	renderKV(writer, "Status", summary.Status)
	renderKV(writer, "Template", summary.Template)
	renderKV(writer, "RootFS", summary.RootFS)
	renderKV(writer, "Network", summary.Net0)
	renderKV(writer, "SSH Key", summary.SSHKeyPath)
	renderKV(writer, "Task", summary.UPID)
}

func RenderConfigSummary(writer io.Writer, summary ConfigSummary) {
	fmt.Fprintln(writer, okStyle.Render("Configuration Saved"))
	renderKV(writer, "Path", summary.Path)
	renderKV(writer, "Storage", summary.DefaultStorage)
	renderKV(writer, "Image", summary.DefaultImage)
	renderKV(writer, "Disk", summary.DefaultDiskSize)
	renderKV(writer, "SSH Key", summary.DefaultSSHPublicKeyPath)
	renderKV(writer, "Mode", summary.NetworkMode)
	if summary.Bridge != "" {
		renderKV(writer, "Bridge", summary.Bridge)
	}
	if summary.Gateway != "" {
		renderKV(writer, "Gateway", summary.Gateway)
	}
	if summary.Netmask != "" {
		renderKV(writer, "Netmask", summary.Netmask)
	}
	if summary.RangeStart != "" || summary.RangeEnd != "" {
		renderKV(writer, "Range", strings.TrimSpace(summary.RangeStart+" - "+summary.RangeEnd))
	}
}

func NewConfigSummary(cfg *config.Config, path string) ConfigSummary {
	return ConfigSummary{
		Path:                    path,
		DefaultStorage:          cfg.DefaultStorage,
		DefaultImage:            cfg.DefaultImage,
		DefaultDiskSize:         cfg.DefaultDiskSize,
		DefaultSSHPublicKeyPath: cfg.DefaultSSHPublicKeyPath,
		NetworkMode:             cfg.Network.Mode,
		Bridge:                  cfg.Network.Bridge,
		Gateway:                 cfg.Network.Gateway,
		Netmask:                 cfg.Network.Netmask,
		RangeStart:              cfg.Network.RangeStart,
		RangeEnd:                cfg.Network.RangeEnd,
	}
}

func renderKV(writer io.Writer, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}

	fmt.Fprintf(writer, "%s %s\n", labelStyle.Render(label+":"), valueStyle.Render(value))
}

func formatDuration(seconds int64) string {
	if seconds <= 0 {
		return "-"
	}

	d := time.Duration(seconds) * time.Second
	if d < time.Hour {
		return d.Truncate(time.Minute).String()
	}

	if d < 24*time.Hour {
		return d.Truncate(time.Minute).String()
	}

	days := int(d.Hours()) / 24
	rest := d - time.Duration(days)*24*time.Hour
	return fmt.Sprintf("%dd %s", days, rest.Truncate(time.Minute))
}

func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return "-"
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

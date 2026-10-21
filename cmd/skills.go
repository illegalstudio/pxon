package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"pxon/internal/wizard"
	pxonskills "pxon/skills"
)

type skillsInstallOptions struct {
	force bool
}

type skillInstallTarget struct {
	Key   string
	Label string
	Path  string
}

type skillInstallItem struct {
	Target string                   `json:"target"`
	Path   string                   `json:"path"`
	Status pxonskills.InstallStatus `json:"status,omitempty"`
	Error  string                   `json:"error,omitempty"`
}

type skillsInstallResult struct {
	Name          string             `json:"name"`
	Installations []skillInstallItem `json:"installations"`
}

var skillsInstallOpts skillsInstallOptions

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage the bundled AI agent skill",
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the PXON skill for AI agents",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find user home directory: %w", err)
		}

		targets := skillInstallTargets(home)
		requested, _ := cmd.Flags().GetStringArray("target")
		if cmd.Flags().Changed("target") {
			targets, err = filterSkillTargets(targets, requested)
		} else if !jsonEnabled() {
			var labels []string
			labels, err = wizard.MultiSelect("Select the AI agent skill destinations:", targetLabels(targets), []int{0, 1})
			if err != nil {
				return fmt.Errorf("select skill destinations (use --target for non-interactive installation): %w", err)
			}
			targets = selectedTargets(targets, labels)
		}
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			return fmt.Errorf("no skill target selected")
		}

		result, installErr := installSkillTargets(targets, skillsInstallOpts.force)
		if jsonEnabled() {
			return writeJSONResult(cmd, result, installErr)
		}
		writeSkillsInstallResult(cmd, result)
		return installErr
	},
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsInstallCmd.Flags().StringArray("target", nil, "Install only to this destination (agents, claude); repeatable")
	skillsInstallCmd.Flags().BoolVar(
		&skillsInstallOpts.force,
		"force",
		false,
		"Replace an existing PXON skill that differs from the bundled version",
	)
}

func skillInstallTargets(home string) []skillInstallTarget {
	return []skillInstallTarget{
		{Key: "agents", Label: "~/.agents/skills", Path: pxonskills.AgentSkillsInstallPath(home)},
		{Key: "claude", Label: "~/.claude/skills", Path: pxonskills.ClaudeSkillsInstallPath(home)},
	}
}

func targetLabels(targets []skillInstallTarget) []string {
	labels := make([]string, len(targets))
	for index, target := range targets {
		labels[index] = target.Label
	}
	return labels
}

func selectedTargets(targets []skillInstallTarget, labels []string) []skillInstallTarget {
	selected := make(map[string]bool, len(labels))
	for _, label := range labels {
		selected[label] = true
	}

	result := make([]skillInstallTarget, 0, len(labels))
	for _, target := range targets {
		if selected[target.Label] {
			result = append(result, target)
		}
	}
	return result
}

func installSkillTargets(targets []skillInstallTarget, force bool) (skillsInstallResult, error) {
	result := skillsInstallResult{
		Name:          pxonskills.Name,
		Installations: make([]skillInstallItem, 0, len(targets)),
	}
	var installErrors []error

	for _, target := range targets {
		status, err := pxonskills.Install(target.Path, force)
		item := skillInstallItem{Target: target.Key, Path: target.Path, Status: status}
		if err != nil {
			item.Error = err.Error()
			installErrors = append(installErrors, fmt.Errorf("%s: %w", target.Path, err))
		}
		result.Installations = append(result.Installations, item)
	}

	return result, errors.Join(installErrors...)
}

func filterSkillTargets(targets []skillInstallTarget, requested []string) ([]skillInstallTarget, error) {
	if len(requested) == 0 {
		return nil, fmt.Errorf("no skill target selected (valid targets: agents, claude)")
	}
	wanted := make(map[string]bool)
	for _, key := range requested {
		key = strings.ToLower(strings.TrimSpace(key))
		if key != "agents" && key != "claude" {
			return nil, fmt.Errorf("unknown skill target %q (valid targets: agents, claude); repeat --target for multiple destinations", key)
		}
		wanted[key] = true
	}
	selected := make([]skillInstallTarget, 0, len(wanted))
	for _, target := range targets {
		if wanted[target.Key] {
			selected = append(selected, target)
		}
	}
	return selected, nil
}

func writeSkillsInstallResult(cmd *cobra.Command, result skillsInstallResult) {
	installed := false
	for _, item := range result.Installations {
		if item.Error != "" {
			continue
		}
		installed = true
		switch item.Status {
		case pxonskills.StatusInstalled:
			fmt.Fprintf(cmd.OutOrStdout(), "Installed PXON skill to %s.\n", item.Path)
		case pxonskills.StatusUpdated:
			fmt.Fprintf(cmd.OutOrStdout(), "Updated PXON skill at %s.\n", item.Path)
		case pxonskills.StatusReplaced:
			fmt.Fprintf(cmd.OutOrStdout(), "Replaced PXON skill at %s.\n", item.Path)
		default:
			fmt.Fprintf(cmd.OutOrStdout(), "PXON skill is already up to date at %s.\n", item.Path)
		}
	}
	if installed {
		fmt.Fprintln(cmd.OutOrStdout(), "Restart an AI agent if the skill does not appear automatically.")
	}
}

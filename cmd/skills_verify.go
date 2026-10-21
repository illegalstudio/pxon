package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	pxonskills "pxon/skills"
)

type skillVerifyItem struct {
	Target string                  `json:"target"`
	Path   string                  `json:"path"`
	Status pxonskills.VerifyStatus `json:"status,omitempty"`
	Error  string                  `json:"error,omitempty"`
}

type skillsVerifyResult struct {
	Name          string            `json:"name"`
	Verifications []skillVerifyItem `json:"verifications"`
}

var skillsVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Check installed skills against the bundled version without changing files",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		targets := skillInstallTargets(home)
		requested, _ := cmd.Flags().GetStringArray("target")
		if cmd.Flags().Changed("target") {
			targets, err = filterSkillTargets(targets, requested)
			if err != nil {
				return err
			}
		}
		result, verifyErr := verifySkillTargets(targets)
		if jsonEnabled() {
			return writeJSONResult(cmd, result, verifyErr)
		}
		for _, item := range result.Verifications {
			status := string(item.Status)
			if item.Error != "" {
				status = item.Error
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s (%s)\n", item.Target, status, item.Path)
		}
		return verifyErr
	},
}

func init() {
	skillsVerifyCmd.Flags().StringArray("target", nil, "Verify only this destination (agents, claude); repeatable")
	skillsCmd.AddCommand(skillsVerifyCmd)
}

func verifySkillTargets(targets []skillInstallTarget) (skillsVerifyResult, error) {
	result := skillsVerifyResult{Name: pxonskills.Name, Verifications: make([]skillVerifyItem, 0, len(targets))}
	var failures []error
	for _, target := range targets {
		status, err := pxonskills.Verify(target.Path)
		item := skillVerifyItem{Target: target.Key, Path: target.Path, Status: status}
		if err != nil {
			item.Error = err.Error()
			failures = append(failures, fmt.Errorf("%s: %w", target.Key, err))
		} else if status.Stale() {
			failures = append(failures, fmt.Errorf("%s skill is %s; run pxon skills install --target %s to update (modified copies require explicit --force)", target.Key, status, target.Key))
		}
		result.Verifications = append(result.Verifications, item)
	}
	return result, errors.Join(failures...)
}

func maybeNoticeStaleSkill(cmd *cobra.Command) {
	if jsonEnabled() || cfg == nil || cfg.SuppressSkillsNotice || !skillNoticeApplies(cmd) {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	result, _ := verifySkillTargets(skillInstallTargets(home))
	for _, item := range result.Verifications {
		if item.Error == "" && item.Status.Stale() {
			fmt.Fprintf(cmd.ErrOrStderr(), "note: the pxon agent skill for %s is %s. Run pxon skills verify and pxon skills install --target %s to update. Local edits require explicit --force. Set suppress_skills_notice: true in the configuration to silence this reminder.\n", item.Target, item.Status, item.Target)
		}
	}
}

func skillNoticeApplies(cmd *cobra.Command) bool {
	if cmd == nil || cmd == rootCmd || cmd.Hidden {
		return false
	}
	if help, _ := cmd.Flags().GetBool("help"); help {
		return false
	}
	for current := cmd; current != nil; current = current.Parent() {
		if current.Name() == "skills" || current.Name() == "completion" {
			return false
		}
	}
	return cmd.Name() != "help" && cmd.Name() != "ssh"
}

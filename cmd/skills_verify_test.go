package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"pxon/internal/config"
	pxonskills "pxon/skills"
)

func TestSkillsJSONInstallAndVerifyWithoutPrompts(t *testing.T) {
	cliTestEnvironment(t, "https://example.invalid")
	result, _, err := runTestCLI(t, "skills", "install", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var installs []skillInstallItem
	if err := json.Unmarshal(result["installations"], &installs); err != nil || len(installs) != 2 {
		t.Fatalf("installations = %s, error = %v", result["installations"], err)
	}
	result, _, err = runTestCLI(t, "skills", "verify", "--target", "claude", "--json")
	var checks []skillVerifyItem
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(result["verifications"], &checks); err != nil || len(checks) != 1 || checks[0].Target != "claude" || checks[0].Status != pxonskills.VerifyUpToDate {
		t.Fatalf("verifications = %s, error = %v", result["verifications"], err)
	}
	if err := os.WriteFile(filepath.Join(installs[0].Path, "SKILL.md"), []byte("local customization\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, _, err = runTestCLI(t, "skills", "verify", "--json")
	if err == nil || result["verifications"] == nil || result["error"] != nil {
		t.Fatalf("stale verification must emit one complete report and fail: %s, %v", result, err)
	}
	result, _, err = runTestCLI(t, "skills", "install", "--target", "agents", "--json")
	if err == nil || result["installations"] == nil {
		t.Fatalf("modified installation must emit its report and fail: %s, %v", result, err)
	}
	if contents, _ := os.ReadFile(filepath.Join(installs[0].Path, "SKILL.md")); string(contents) != "local customization\n" {
		t.Fatal("local customization was overwritten")
	}
}

func TestSkillsTargetValidationAndMissingVerification(t *testing.T) {
	cliTestEnvironment(t, "https://example.invalid")
	for _, target := range []string{"unknown", "agents,claude", ""} {
		result, _, err := runTestCLI(t, "skills", "install", "--target", target, "--json")
		if err == nil || result["error"] == nil {
			t.Fatalf("accepted invalid target %q: %s, %v", target, result, err)
		}
	}
	result, _, err := runTestCLI(t, "skills", "verify", "--json")
	if err != nil || result["verifications"] == nil {
		t.Fatalf("missing skills: %s, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".agents")); !os.IsNotExist(err) {
		t.Fatal("verification created an agent directory")
	}
	selected, err := filterSkillTargets(skillInstallTargets("/test"), []string{"claude", "agents", "claude"})
	if err != nil || len(selected) != 2 || selected[0].Key != "agents" || selected[1].Key != "claude" {
		t.Fatalf("selection = %#v, error = %v", selected, err)
	}
}

func TestVerifySkillTargetsReportsErrorsIndependently(t *testing.T) {
	targets := skillInstallTargets(t.TempDir())
	if _, err := installSkillTargets(targets, false); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(targets[0].Path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targets[0].Path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := verifySkillTargets(targets)
	if err == nil || result.Verifications[0].Error == "" || result.Verifications[1].Status != pxonskills.VerifyUpToDate {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestStaleSkillNoticeRespectsJSONAndSuppression(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := pxonskills.AgentSkillsInstallPath(home)
	if _, err := pxonskills.Install(target, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldCfg, oldJSON := cfg, outputJSON
	t.Cleanup(func() { cfg, outputJSON = oldCfg, oldJSON })
	command := &cobra.Command{Use: "list"}
	var stderr bytes.Buffer
	command.SetErr(&stderr)
	cfg = &config.Config{}
	outputJSON = false
	maybeNoticeStaleSkill(command)
	if !strings.Contains(stderr.String(), "modified") || !strings.Contains(stderr.String(), "suppress_skills_notice") {
		t.Fatalf("notice = %q", stderr.String())
	}
	stderr.Reset()
	outputJSON = true
	maybeNoticeStaleSkill(command)
	outputJSON = false
	cfg.SuppressSkillsNotice = true
	maybeNoticeStaleSkill(command)
	if stderr.Len() != 0 {
		t.Fatalf("unexpected notice: %q", stderr.String())
	}
	for _, command := range []*cobra.Command{skillsInstallCmd, skillsVerifyCmd, sshCmd, rootCmd, {Use: "__complete", Hidden: true}} {
		if skillNoticeApplies(command) {
			t.Errorf("notice applies to %s", command.Name())
		}
	}
}

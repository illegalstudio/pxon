package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillInstallTargets(t *testing.T) {
	home := t.TempDir()
	targets := skillInstallTargets(home)
	if len(targets) != 2 {
		t.Fatalf("target count = %d, want 2", len(targets))
	}

	wants := []string{
		filepath.Join(home, ".agents", "skills", "pxon"),
		filepath.Join(home, ".claude", "skills", "pxon"),
	}
	for index, want := range wants {
		if targets[index].Path != want {
			t.Fatalf("target %d path = %q, want %q", index, targets[index].Path, want)
		}
	}
}

func TestInstallSkillTargetsInstallsBothDestinations(t *testing.T) {
	targets := skillInstallTargets(t.TempDir())
	result, err := installSkillTargets(targets, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installations) != 2 {
		t.Fatalf("installation count = %d, want 2", len(result.Installations))
	}

	for _, target := range targets {
		if _, err := os.Stat(filepath.Join(target.Path, "SKILL.md")); err != nil {
			t.Fatalf("installed SKILL.md at %s: %v", target.Path, err)
		}
	}
}

func TestSelectedTargetsPreservesTargetOrder(t *testing.T) {
	targets := skillInstallTargets(t.TempDir())
	selected := selectedTargets(targets, []string{targets[1].Label, targets[0].Label})
	if len(selected) != 2 {
		t.Fatalf("selected count = %d, want 2", len(selected))
	}
	if selected[0].Label != targets[0].Label || selected[1].Label != targets[1].Label {
		t.Fatalf("selected order = %q, %q", selected[0].Label, selected[1].Label)
	}
}

func TestInstallSkillTargetsKeepsDestinationsIndependent(t *testing.T) {
	targets := skillInstallTargets(t.TempDir())
	if _, err := installSkillTargets(targets[:1], false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targets[0].Path, "SKILL.md"), []byte("custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := installSkillTargets(targets, false)
	if err == nil {
		t.Fatal("expected a conflict for the modified first destination")
	}
	if result.Installations[0].Error == "" {
		t.Fatal("first destination did not report its conflict")
	}
	if result.Installations[1].Error != "" {
		t.Fatalf("second destination failed: %s", result.Installations[1].Error)
	}
	if _, err := os.Stat(filepath.Join(targets[1].Path, "SKILL.md")); err != nil {
		t.Fatalf("second destination was not installed: %v", err)
	}
}

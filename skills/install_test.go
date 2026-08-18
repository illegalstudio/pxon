package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallPaths(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "tmp", "pxon-home")
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "Agent Skills",
			got:  AgentSkillsInstallPath(home),
			want: filepath.Join(home, ".agents", "skills", "pxon"),
		},
		{
			name: "Claude Code",
			got:  ClaudeSkillsInstallPath(home),
			want: filepath.Join(home, ".claude", "skills", "pxon"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("path = %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestInstallNewAndIdempotent(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())

	status, err := Install(destination, false)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusInstalled {
		t.Fatalf("status = %q, want %q", status, StatusInstalled)
	}

	if _, err := os.Stat(filepath.Join(destination, "SKILL.md")); err != nil {
		t.Fatalf("installed SKILL.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "agents", "openai.yaml")); err != nil {
		t.Fatalf("installed agents/openai.yaml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, markerFileName)); err != nil {
		t.Fatalf("installed management marker: %v", err)
	}

	status, err = Install(destination, false)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusUnchanged {
		t.Fatalf("status = %q, want %q", status, StatusUnchanged)
	}
}

func TestInstallRefusesModifiedSkillWithoutForce(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())
	if _, err := Install(destination, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "SKILL.md"), []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(destination, false); err == nil {
		t.Fatal("expected modified skill conflict")
	}
}

func TestInstallUpdatesUnmodifiedManagedSkill(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())
	if _, err := Install(destination, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "SKILL.md"), []byte("previous bundled skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	previousDigest, err := digestInstalledSkill(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeMarker(destination, previousDigest); err != nil {
		t.Fatal(err)
	}

	status, err := Install(destination, false)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusUpdated {
		t.Fatalf("status = %q, want %q", status, StatusUpdated)
	}

	status, err = Install(destination, false)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusUnchanged {
		t.Fatalf("status = %q, want %q", status, StatusUnchanged)
	}
}

func TestInstallForceReplacesModifiedSkill(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())
	if _, err := Install(destination, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "extra.txt"), []byte("custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := Install(destination, true)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusReplaced {
		t.Fatalf("status = %q, want %q", status, StatusReplaced)
	}
	if _, err := os.Stat(filepath.Join(destination, "extra.txt")); !os.IsNotExist(err) {
		t.Fatalf("extra file still exists or stat failed: %v", err)
	}

	status, err = Install(destination, false)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusUnchanged {
		t.Fatalf("status = %q, want %q", status, StatusUnchanged)
	}
}

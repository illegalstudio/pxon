package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyInstalledStatesAndReadOnlyBehavior(t *testing.T) {
	destination := AgentSkillsInstallPath(t.TempDir())
	check := func(want VerifyStatus) {
		t.Helper()
		got, err := Verify(destination)
		if err != nil || got != want {
			t.Fatalf("Verify = %q, %v; want %q", got, err, want)
		}
	}
	check(VerifyNotInstalled)
	if _, err := os.Stat(filepath.Dir(destination)); !os.IsNotExist(err) {
		t.Fatal("verification created a missing directory")
	}
	if _, err := Install(destination, false); err != nil {
		t.Fatal(err)
	}
	check(VerifyUpToDate)
	if err := os.Remove(filepath.Join(destination, markerFileName)); err != nil {
		t.Fatal(err)
	}
	check(VerifyUpToDate)
	if _, err := os.Stat(filepath.Join(destination, markerFileName)); !os.IsNotExist(err) {
		t.Fatal("verification wrote a missing marker")
	}
	if err := os.WriteFile(filepath.Join(destination, "SKILL.md"), []byte("previous bundled content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := digestInstalledSkill(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeMarker(destination, digest); err != nil {
		t.Fatal(err)
	}
	check(VerifyOutdated)
	if err := os.WriteFile(filepath.Join(destination, "SKILL.md"), []byte("local changes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	check(VerifyModified)
	if _, err := Install(destination, false); err == nil {
		t.Fatal("modified skill was overwritten")
	}
}

func TestVerifyRejectsNonDirectoriesAndSymlinks(t *testing.T) {
	home := t.TempDir()
	file := filepath.Join(home, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, "link")
	if err := os.Symlink(home, link); err != nil {
		t.Fatal(err)
	}
	for _, destination := range []string{"", file, link} {
		if _, err := Verify(destination); err == nil {
			t.Errorf("accepted invalid destination %q", destination)
		}
	}
}

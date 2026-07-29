package knownhosts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindAndRemove(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}

	path := filepath.Join(t.TempDir(), "known_hosts")
	content := strings.Join([]string{
		"10.0.0.8 ssh-ed25519 AAAATEST pxon-ip",
		"box.test,alias.test ssh-ed25519 AAAATEST pxon-name",
		"unrelated.test ssh-ed25519 AAAATEST keep",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	matches, err := Find(path, []string{"box.test", "10.0.0.8", "box.test", ""})
	if err != nil {
		t.Fatal(err)
	}
	if matches.Rows != 2 {
		t.Fatalf("Rows = %d, want 2", matches.Rows)
	}
	if len(matches.Hosts) != 2 {
		t.Fatalf("Hosts = %v, want two matching hosts", matches.Hosts)
	}

	if err := Remove(path, matches.Hosts); err != nil {
		t.Fatal(err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(updated), "10.0.0.8") {
		t.Fatal("IP entry was not removed")
	}
	if strings.Contains(string(updated), "box.test") {
		t.Fatal("hostname entry was not removed")
	}
	if !strings.Contains(string(updated), "unrelated.test") {
		t.Fatal("unrelated entry was removed")
	}
}

func TestFindMissingFile(t *testing.T) {
	matches, err := Find(filepath.Join(t.TempDir(), "known_hosts"), []string{"box.test"})
	if err != nil {
		t.Fatal(err)
	}
	if matches.Rows != 0 || len(matches.Hosts) != 0 {
		t.Fatalf("matches = %+v, want no matches", matches)
	}
}

func TestFindHashedHost(t *testing.T) {
	sshKeygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		t.Skip("ssh-keygen not available")
	}

	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte("box.test ssh-ed25519 AAAATEST pxon\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if output, err := exec.Command(sshKeygen, "-H", "-f", path).CombinedOutput(); err != nil {
		t.Fatalf("hash fixture: %s: %v", strings.TrimSpace(string(output)), err)
	}

	matches, err := Find(path, []string{"box.test"})
	if err != nil {
		t.Fatal(err)
	}
	if matches.Rows != 1 || len(matches.Hosts) != 1 {
		t.Fatalf("matches = %+v, want one hashed match", matches)
	}
}

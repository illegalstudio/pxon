package config

import "testing"

func TestSaveAndLoadPreservesSkillNoticeSetting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := &Config{Endpoint: "https://example.invalid/api2/json", TokenID: "test@pve!pxon", TokenSecret: "test-secret", SuppressSkillsNotice: true, DefaultImage: "lxc:900"}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil || !loaded.SuppressSkillsNotice || loaded.DefaultImage != "lxc:900" {
		t.Fatalf("configuration = %#v, error = %v", loaded, err)
	}
	t.Setenv("PXON_SUPPRESS_SKILLS_NOTICE", "false")
	loaded, err = Load()
	if err != nil || loaded.SuppressSkillsNotice {
		t.Fatalf("environment override = %#v, error = %v", loaded, err)
	}
}

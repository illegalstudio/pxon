package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func resetCommandFlags(command *cobra.Command) {
	reset := func(flag *pflag.Flag) {
		if value, ok := flag.Value.(pflag.SliceValue); ok {
			_ = value.Replace(nil)
		} else {
			_ = flag.Value.Set(flag.DefValue)
		}
		flag.Changed = false
	}
	command.Flags().VisitAll(reset)
	command.PersistentFlags().VisitAll(reset)
	for _, child := range command.Commands() {
		resetCommandFlags(child)
	}
}

func cliTestEnvironment(t *testing.T, endpoint string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	for key, value := range map[string]string{
		"PXON_ENDPOINT": endpoint, "PXON_TOKEN_ID": "test@pve!pxon", "PXON_TOKEN_SECRET": "test-secret",
		"PXON_DEFAULT_STORAGE": "local-lvm", "PXON_DEFAULT_DISK_SIZE": "8",
		"PXON_DEFAULT_IMAGE": "local:vztmpl/ubuntu.tar.zst", "PXON_DEFAULT_NET0": "name=eth0,bridge=vmbr0,ip=192.0.2.20/24,gw=192.0.2.1",
		"PXON_DEFAULT_SSH_PUBLIC_KEY_PATH": "", "PXON_DEFAULT_PASSWORD": "", "PXON_SUPPRESS_SKILLS_NOTICE": "false",
	} {
		t.Setenv(key, value)
	}
}

func runTestCLI(t *testing.T, args ...string) (map[string]json.RawMessage, string, error) {
	t.Helper()
	resetCommandFlags(rootCmd)
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		resetCommandFlags(rootCmd)
	})
	err := Execute()
	var result map[string]json.RawMessage
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("invalid JSON result %q: %v (command error %v)", stdout.String(), decodeErr, err)
	}
	return result, stderr.String(), err
}

package cmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"pxon/internal/proxmox"
)

func TestParseSSHInvocation(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		argsLenAtDash int
		want          sshInvocation
		wantErr       string
	}{
		{
			name:          "interactive selection",
			argsLenAtDash: -1,
		},
		{
			name:          "targeted interactive shell",
			args:          []string{" web "},
			argsLenAtDash: -1,
			want:          sshInvocation{identifier: "web"},
		},
		{
			name:          "targeted remote command",
			args:          []string{"web", "sh", "-lc", "printf '%s\\n' hello"},
			argsLenAtDash: 1,
			want: sshInvocation{
				identifier:    "web",
				remoteCommand: []string{"sh", "-lc", "printf '%s\\n' hello"},
			},
		},
		{
			name:          "interactive target selection with remote command",
			args:          []string{"hostname"},
			argsLenAtDash: 0,
			want:          sshInvocation{remoteCommand: []string{"hostname"}},
		},
		{
			name:          "remote command without separator",
			args:          []string{"web", "hostname"},
			argsLenAtDash: -1,
			wantErr:       "must follow --",
		},
		{
			name:          "missing remote command",
			args:          []string{"web"},
			argsLenAtDash: 1,
			wantErr:       "required after --",
		},
		{
			name:          "too many target arguments",
			args:          []string{"web", "extra", "hostname"},
			argsLenAtDash: 2,
			wantErr:       "at most one",
		},
		{
			name:          "empty target",
			args:          []string{"  "},
			argsLenAtDash: -1,
			wantErr:       "name or VMID is required",
		},
		{
			name:          "empty remote command",
			args:          []string{"web", "  "},
			argsLenAtDash: 1,
			wantErr:       "required after --",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSSHInvocation(tt.args, tt.argsLenAtDash)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("invocation = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSSHArgsLenAtDashFromCobra(t *testing.T) {
	var got sshInvocation
	command := &cobra.Command{
		Use:  "ssh",
		Args: validateSSHArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			got, err = parseSSHInvocation(args, cmd.ArgsLenAtDash())
			return err
		},
	}
	command.SetArgs([]string{"web", "--", "sh", "-lc", "echo hello"})

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}

	want := sshInvocation{
		identifier:    "web",
		remoteCommand: []string{"sh", "-lc", "echo hello"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("invocation = %#v, want %#v", got, want)
	}
}

func TestSSHCommandArgs(t *testing.T) {
	got := sshCommandArgs("192.0.2.10", []string{"sh", "-lc", "echo hello"})
	want := []string{"ssh", "root@192.0.2.10", "sh", "-lc", "echo hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestFindContainer(t *testing.T) {
	containers := []proxmox.Container{
		{Name: "web", VMID: 101},
		{Name: "worker", VMID: 102},
	}

	byName, err := findContainer(containers, "WEB")
	if err != nil {
		t.Fatal(err)
	}
	if byName.VMID != 101 {
		t.Fatalf("VMID = %d, want 101", byName.VMID)
	}

	byVMID, err := findContainer(containers, "102")
	if err != nil {
		t.Fatal(err)
	}
	if byVMID.Name != "worker" {
		t.Fatalf("Name = %q, want worker", byVMID.Name)
	}
}

func TestFindContainerRejectsAmbiguousName(t *testing.T) {
	containers := []proxmox.Container{
		{Name: "web", VMID: 101},
		{Name: "WEB", VMID: 102},
	}

	if _, err := findContainer(containers, "web"); err == nil {
		t.Fatal("expected ambiguous name error")
	}
}

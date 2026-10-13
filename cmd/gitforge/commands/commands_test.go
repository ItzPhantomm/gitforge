package commands

import (
	"bytes"
	"testing"
)

func TestCommandCreation(t *testing.T) {
	cmds := []*struct {
		name string
		use  string
	}{
		{"status", NewStatusCmd().Use},
		{"branch", NewBranchCmd().Use},
		{"log", NewLogCmd().Use},
		{"diff", NewDiffCmd().Use},
		{"doctor", NewDoctorCmd().Use},
		{"version", NewVersionCmd().Use},
		{"init", NewInitCmd().Use},
		{"clone", NewCloneCmd().Use},
		{"commit", NewCommitCmd().Use},
		{"remote", NewRemoteCmd().Use},
		{"stash", NewStashCmd().Use},
		{"tag", NewTagCmd().Use},
		{"config", NewConfigCmd().Use},
		{"plugin", NewPluginCmd().Use},
		{"web", NewWebCmd().Use},
	}

	for _, c := range cmds {
		if c.use == "" {
			t.Errorf("command %s has empty Use field", c.name)
		}
	}
}

func TestVersionCommandOutput(t *testing.T) {
	cmd := NewVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("failed to execute version command: %v", err)
	}
}

func TestDoctorCommandOutput(t *testing.T) {
	cmd := NewDoctorCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("failed to execute doctor command: %v", err)
	}
}

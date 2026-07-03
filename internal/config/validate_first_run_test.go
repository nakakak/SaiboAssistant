package config

import "testing"

func TestValidate_firstRunAllowsEmptyServerURL(t *testing.T) {
	tok := true
	c := &Config{
		FirstRun:  true,
		ServerURL: "",
		Channels: struct {
			OpenClaw *bool `yaml:"openclaw,omitempty"`
			Telnet   *bool `yaml:"telnet,omitempty"`
		}{OpenClaw: &tok, Telnet: &tok},
	}
	if err := Validate(c); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidate_runtimeRequiresServerURL(t *testing.T) {
	c := &Config{
		FirstRun:  false,
		ServerURL: "",
		DeviceMAC: "aa:bb:cc:dd:ee:ff",
	}
	if err := Validate(c); err == nil {
		t.Fatal("expected error for empty server_url")
	}
}

func TestDictationSubtitle_defaultFalse(t *testing.T) {
	c := &Config{}
	if c.DictationSubtitle() {
		t.Fatal("expected subtitle default false")
	}
}

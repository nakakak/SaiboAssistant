package config

import "testing"

func TestPrepareForUse_gatewayWSLocal(t *testing.T) {
	c := &Config{
		ServerURL: "ws://127.0.0.1:8084/bridge/connector",
		DeviceMAC: "aa:bb:cc:dd:ee:ff",
		Token:     wizardPendingToken,
	}
	tok := true
	c.Channels.OpenClaw = &tok
	c.OpenClaw.Mode = "gateway_ws"
	c.OpenClaw.BaseURL = "http://127.0.0.1:18789"
	c.OpenClaw.GatewayToken = "secret"

	PrepareForUse(c)
	if c.Token != "" {
		t.Fatalf("token=%q want empty", c.Token)
	}
	if !c.OpenClaw.GatewaySameHost {
		t.Fatal("GatewaySameHost want true")
	}
	if !c.OpenClaw.GatewayDialLoopback {
		t.Fatal("GatewayDialLoopback want true for loopback")
	}
}

func TestPrepareForUse_gatewayWSRemote(t *testing.T) {
	c := &Config{
		ServerURL: "ws://127.0.0.1:8084/bridge/connector",
		DeviceMAC: "aa:bb:cc:dd:ee:ff",
	}
	tok := true
	c.Channels.OpenClaw = &tok
	c.OpenClaw.Mode = "gateway_ws"
	c.OpenClaw.BaseURL = "http://192.168.1.10:18789"
	c.OpenClaw.GatewayToken = "secret"
	c.OpenClaw.GatewayDialLoopback = true

	PrepareForUse(c)
	if !c.OpenClaw.GatewaySameHost {
		t.Fatal("GatewaySameHost want true")
	}
	if c.OpenClaw.GatewayDialLoopback {
		t.Fatal("GatewayDialLoopback want false for remote host")
	}
}

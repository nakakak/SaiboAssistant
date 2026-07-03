package driver

// applyGatewayWSHostDefaults：本机 loopback 网关须用 Control UI 握手，否则 token 连接会被清空 scopes。
func applyGatewayWSHostDefaults(wsURL string, sameHost, dialLoopback *bool) {
	if sameHost == nil || dialLoopback == nil {
		return
	}
	if !isWSHostLoopback(wsURL) {
		return
	}
	*sameHost = true
	*dialLoopback = true
}

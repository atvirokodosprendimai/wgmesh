package wireguard

// FullConfigToConfig converts a FullConfig to a Config structure.
func FullConfigToConfig(full *FullConfig) *Config {
	config := &Config{
		Interface: Interface{
			PrivateKey: full.Interface.PrivateKey,
			Address:    full.Interface.Address,
			ListenPort: full.Interface.ListenPort,
		},
		Peers: make(map[string]Peer),
	}

	for _, p := range full.Peers {
		config.Peers[p.PublicKey] = Peer{
			PublicKey:           p.PublicKey,
			Endpoint:            p.Endpoint,
			AllowedIPs:          p.AllowedIPs,
			PersistentKeepalive: p.PersistentKeepalive,
		}
	}

	return config
}

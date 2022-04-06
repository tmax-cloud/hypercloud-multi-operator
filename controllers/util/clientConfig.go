package util

func GetClientConfig(client string, clientPrefix string) ClientConfig {
	clientConfig := map[string]ClientConfig{
		"kibana": {
			ClientId:                  clientPrefix + "kibana",
			Secret:                    "tmax-client-secret",
			DirectAccessGrantsEnabled: true,
			ImplicitFlowEnabled:       false,
			RedirectUris:              []string{"*"},
		},
		"grafana": {
			ClientId:                  clientPrefix + "grafana",
			Secret:                    "tmax-client-secret",
			DirectAccessGrantsEnabled: true,
			ImplicitFlowEnabled:       false,
			RedirectUris:              []string{"*"},
		},
		"kiali": {
			ClientId:                  clientPrefix + "kiali",
			Secret:                    "tmax-client-secret",
			DirectAccessGrantsEnabled: true,
			ImplicitFlowEnabled:       true,
			RedirectUris:              []string{"*"},
		},
		// "jaeger": {
		// 	ClientId:                  clientPrefix + "jaeger",
		// 	Secret:                    "tmax-client-secret",
		// 	DirectAccessGrantsEnabled: true,
		// 	ImplicitFlowEnabled:       false,
		// 	RedirectUris:              []string{"*"},
		// },
		// "hyperregistry": {
		// 	ClientId:                  clientPrefix + "hyperregistry",
		// 	Secret:                    "tmax-client-secret",
		// 	DirectAccessGrantsEnabled: true,
		// 	ImplicitFlowEnabled:       false,
		// 	RedirectUris:              []string{"*"},
		// },
	}

	if config, ok := clientConfig[client]; ok {
		return config
	}

	return ClientConfig{}
}

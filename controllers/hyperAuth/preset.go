/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hyperAuth

import "os"

func GetClientConfigPreset(prefix string) []ClientConfig {
	configs := []ClientConfig{
		{
			ClientId:                  prefix + "kibana",
			Secret:                    os.Getenv("AUTH_CLIENT_SECRET"),
			DirectAccessGrantsEnabled: true,
			ImplicitFlowEnabled:       false,
			RedirectUris:              []string{"*"},
		},
		{
			ClientId:                  prefix + "grafana",
			Secret:                    os.Getenv("AUTH_CLIENT_SECRET"),
			DirectAccessGrantsEnabled: true,
			ImplicitFlowEnabled:       false,
			RedirectUris:              []string{"*"},
		},
		{
			ClientId:                  prefix + "kiali",
			Secret:                    os.Getenv("AUTH_CLIENT_SECRET"),
			DirectAccessGrantsEnabled: true,
			ImplicitFlowEnabled:       true,
			RedirectUris:              []string{"*"},
		},
		{
			ClientId:                  prefix + "jaeger",
			Secret:                    os.Getenv("AUTH_CLIENT_SECRET"),
			DirectAccessGrantsEnabled: true,
			ImplicitFlowEnabled:       false,
			RedirectUris:              []string{"*"},
		},
		{
			ClientId:                  prefix + "hyperregistry",
			Secret:                    os.Getenv("AUTH_CLIENT_SECRET"),
			DirectAccessGrantsEnabled: true,
			ImplicitFlowEnabled:       false,
			RedirectUris:              []string{"*"},
		},
		{
			ClientId:                  prefix + "opensearch",
			Secret:                    os.Getenv("AUTH_CLIENT_SECRET"),
			DirectAccessGrantsEnabled: true,
			ImplicitFlowEnabled:       false,
			RedirectUris:              []string{"*"},
		},
	}

	return configs
}

func GetMappingProtocolMapperToClientConfigPreset(prefix string) []ClientLevelProtocolMapperConfig {
	configs := []ClientLevelProtocolMapperConfig{
		{
			ClientId: prefix + "kibana",
			ProtocolMapper: ProtocolMapperConfig{
				Name:           "kibana",
				Protocol:       PROTOCOL_MAPPER_CONFIG_PROTOCOL_OPENID_CONNECT,
				ProtocolMapper: PROTOCOL_MAPPER_CONFIG_PROTOCOL_NAME_AUDIENCE,
				Config: MapperConfig{
					IncludedClientAudience: prefix + "kibana",
					IdTokenClaim:           false,
					AccessTokenClaim:       true,
				},
			},
		},
		{
			ClientId: prefix + "jaeger",
			ProtocolMapper: ProtocolMapperConfig{
				Name:           "jaeger",
				Protocol:       PROTOCOL_MAPPER_CONFIG_PROTOCOL_OPENID_CONNECT,
				ProtocolMapper: PROTOCOL_MAPPER_CONFIG_PROTOCOL_NAME_AUDIENCE,
				Config: MapperConfig{
					IncludedClientAudience: prefix + "jaeger",
					IdTokenClaim:           false,
					AccessTokenClaim:       true,
				},
			},
		},
		{
			ClientId: prefix + "hyperregistry",
			ProtocolMapper: ProtocolMapperConfig{
				Name:           "group",
				Protocol:       PROTOCOL_MAPPER_CONFIG_PROTOCOL_OPENID_CONNECT,
				ProtocolMapper: PROTOCOL_MAPPER_CONFIG_PROTOCOL_NAME_GROUP_MEMBERSHIP,
				Config: MapperConfig{
					ClaimName:          "group",
					FullPath:           true,
					IdTokenClaim:       true,
					AccessTokenClaim:   true,
					UserInfoTokenClaim: true,
				},
			},
		},
		{
			ClientId: prefix + "opensearch",
			ProtocolMapper: ProtocolMapperConfig{
				Name:           "client roles",
				Protocol:       PROTOCOL_MAPPER_CONFIG_PROTOCOL_OPENID_CONNECT,
				ProtocolMapper: PROTOCOL_MAPPER_CONFIG_PROTOCOL_NAME_USER_CLIENT_ROLE,
				Config: MapperConfig{
					Multivalued:        true,
					ClaimName:          "roles",
					JsonType:           "String",
					IdTokenClaim:       true,
					AccessTokenClaim:   true,
					UserInfoTokenClaim: true,
				},
			},
		},
	}

	return configs
}

func GetClientLevelRoleConfigPreset(prefix string) []ClientLevelRoleConfig {
	configs := []ClientLevelRoleConfig{
		{
			ClientId: prefix + "kibana",
			Role: RoleConfig{
				Name: "kibana-manager",
			},
		},
		{
			ClientId: prefix + "jaeger",
			Role: RoleConfig{
				Name: "jaeger-manager",
			},
		},
		{
			ClientId: prefix + "opensearch",
			Role: RoleConfig{
				Name: "opensearch-admin",
			},
		},
		{
			ClientId: prefix + "opensearch",
			Role: RoleConfig{
				Name: "opensearch-developer",
			},
		},
		{
			ClientId: prefix + "opensearch",
			Role: RoleConfig{
				Name: "opensearch-guest",
			},
		},
	}

	return configs
}

func GetClientScopeMappingPreset(prefix string) []ClientScopeMappingConfig {
	configs := []ClientScopeMappingConfig{
		{
			ClientId: prefix + "kiali",
			ClientScope: ClientScopeConfig{
				Name: "kubernetes",
			},
		},
	}

	return configs
}

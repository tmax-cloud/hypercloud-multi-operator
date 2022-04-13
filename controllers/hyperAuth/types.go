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

type ClientConfig struct {
	Id                        string   `json:"id,omitempty"`
	ClientId                  string   `json:"clientId,omitempty"`
	Secret                    string   `json:"secret,omitempty"`
	DirectAccessGrantsEnabled bool     `json:"directAccessGrantsEnabled,omitempty"`
	ImplicitFlowEnabled       bool     `json:"implicitFlowEnabled,omitempty"`
	RedirectUris              []string `json:"redirectUris,omitempty"`
	// ServiceAccountsEnabled    bool     `json:"serviceAccountsEnabled,omitempty"`
}

// func (config ClientConfig) IsExist() bool {
// 	return config.Id == ""
// }

// func (config ClientConfig) IsEmpty() bool {
// 	return config.ClientId == ""
// }

// func (source ClientConfig) IsEqual(dest ClientConfig) bool {
// 	return source.ClientId == dest.ClientId
// }

type ClientLevelProtocolMapperConfig struct {
	ClientId       string
	ProtocolMapper ProtocolMapperConfig
}

type ProtocolMapperConfig struct {
	Name           string       `json:"name,omitempty"`
	Protocol       string       `json:"protocol,omitempty"`
	ProtocolMapper string       `json:"protocolMapper,omitempty"`
	Config         MapperConfig `json:"config,omitempty"`
}

type MapperConfig struct {
	IncludedClientAudience string `json:"included.client.audience,omitempty"`
	IncludedCustomAudience string `json:"included.custom.audience,omitempty"`
	ClaimName              string `json:"claim.name,omitempty"`
	IdTokenClaim           bool   `json:"id.token.claim,omitempty"`
	FullPath               bool   `json:"full.path,omitempty"`
	AccessTokenClaim       bool   `json:"access.token.claim,omitempty"`
	UserInfoTokenClaim     bool   `json:"userinfo.token.claim,omitempty"`
}

type ClientLevelRoleConfig struct {
	ClientId string
	Role     RoleConfig
}

type RoleConfig struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type UserConfig struct {
	Id string `json:"id,omitempty"`
}

type ClientScopeMappingConfig struct {
	ClientId    string
	ClientScope ClientScopeConfig
}

type ClientScopeConfig struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

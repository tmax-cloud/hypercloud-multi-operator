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

const (
	// admin api
	KEYCLOAK_ADMIN_SERVICE_GET_TOKEN                      = "/auth/realms/master/protocol/openid-connect/token"
	KEYCLOAK_ADMIN_SERVICE_GET_CLIENTS                    = "/auth/admin/realms/tmax/clients"
	KEYCLOAK_ADMIN_SERVICE_CREATE_CLIENT                  = "/auth/admin/realms/tmax/clients"
	KEYCLOAK_ADMIN_SERVICE_DELETE_CLIENT                  = "/auth/admin/realms/tmax/clients/@@id@@"
	KEYCLOAK_ADMIN_SERVICE_CREATE_CLIENT_PROTOCOL_MAPPERS = "/auth/admin/realms/tmax/clients/@@id@@/protocol-mappers/models"
	KEYCLOAK_ADMIN_SERVICE_CREATE_ROLES                   = "/auth/admin/realms/tmax/clients/@@id@@/roles"
	KEYCLOAK_ADMIN_SERVICE_GET_ROLE_BY_NAME               = "/auth/admin/realms/tmax/clients/@@id@@/roles/@@roleName@@"
	KEYCLOAK_ADMIN_SERVICE_ADD_ROLE_TO_USER               = "/auth/admin/realms/tmax/users/@@userId@@/role-mappings/clients/@@id@@"
	KEYCLOAK_ADMIN_SERVICE_GET_CLIENT_SCOPES              = "/auth/admin/realms/tmax/client-scopes"
	KEYCLOAK_ADMIN_SERVICE_ADD_CLIENT_SCOPE_TO_CLIENT     = "/auth/admin/realms/tmax/clients/@@id@@/optional-client-scopes/@@clientScopeId@@"

	// hyperauth api
	HYPERAUTH_SERVICE_GET_USER_ID_BY_EMAIL = "/auth/realms/tmax/user/@@userEmail@@?token=@@token@@"
)

const (
	PROTOCOL_MAPPER_CONFIG_PROTOCOL_OPENID_CONNECT        = "openid-connect"
	PROTOCOL_MAPPER_CONFIG_PROTOCOL_NAME_AUDIENCE         = "oidc-audience-mapper"
	PROTOCOL_MAPPER_CONFIG_PROTOCOL_NAME_GROUP_MEMBERSHIP = "oidc-group-membership-mapper"
)

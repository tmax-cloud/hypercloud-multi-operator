package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type ClientConfig struct {
	Id                        string   `json:"id,omitempty"`
	ClientId                  string   `json:"clientId,omitempty"`
	Secret                    string   `json:"secret,omitempty"`
	DirectAccessGrantsEnabled bool     `json:"directAccessGrantsEnabled,omitempty"`
	ImplicitFlowEnabled       bool     `json:"implicitFlowEnabled,omitempty"`
	RedirectUris              []string `json:"redirectUris,omitempty"`
	// ServiceAccountsEnabled    bool     `json:"serviceAccountsEnabled,omitempty"`
}

// func (clientConfig ClientConfig) IsExist() bool {
// 	return clientConfig.Id == ""
// }

func (clientConfig ClientConfig) IsEmpty() bool {
	return clientConfig.ClientId == ""
}

// func (source ClientConfig) IsEqual(dest ClientConfig) bool {
// 	return source.ClientId == dest.ClientId
// }

type ProtocolMapperConfig struct {
	Name           string       `json:"name,omitempty"`
	Protocol       string       `json:"protocol,omitempty"`
	ProtocolMapper string       `json:"protocolMapper,omitempty"`
	Config         MapperConfig `json:"config,omitempty"`
}

type MapperConfig struct {
	IncludedClientAudience string `json:"included.client.audience,omitempty"`
	IdTokenClaim           bool   `json:"id.token.claim,omitempty"`
	AccessTokenClaim       bool   `json:"access.token.claim,omitempty"`
	UserInfoTokenClaim     bool   `json:"userinfo.token.claim,omitempty"`
}

type RoleConfig struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type UserConfig struct {
	Id string `json:"id,omitempty"`
}

type ClientScopeConfig struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// defunct
// func SetHyperAuthURL(serviceName string) string {
// 	hyperauthHttpPort := "80"
// 	if os.Getenv("HYPERAUTH_HTTP_PORT") != "" {
// 		hyperauthHttpPort = os.Getenv("HYPERAUTH_HTTP_PORT")
// 	}
// 	return "http://hyperauth." + os.Getenv("HC_DOMAIN") + ":" + hyperauthHttpPort + "/" + serviceName
// }

func SetSecureHyperAuthURL(serviceName string, urlParameter map[string]string) string {
	for key, value := range urlParameter {
		serviceName = strings.Replace(serviceName, "@@"+key+"@@", value, 1)
	}
	return "https://hyperauth." + os.Getenv("HC_DOMAIN") + serviceName
}

func GetHyperauthAdminToken(secret *corev1.Secret) (string, error) {
	// Make Body for Content-Type (application/x-www-form-urlencoded)
	id, password := string(secret.Data["HYPERAUTH_ADMIN"]), string(secret.Data["HYPERAUTH_PASSWORD"])
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", id)
	data.Set("password", password)
	data.Set("client_id", "admin-cli")

	// Make Request Object
	url := SetSecureHyperAuthURL(KEYCLOAK_ADMIN_SERVICE_GET_TOKEN, nil)
	playload := strings.NewReader(data.Encode())
	req, err := http.NewRequest(http.MethodPost, url, playload)
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// Request with Client Object
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Result
	bytes, _ := ioutil.ReadAll(resp.Body)
	str := string(bytes)

	var resultJson map[string]interface{}
	if err := json.Unmarshal([]byte(str), &resultJson); err != nil {
		return "", err
	}
	accessToken := resultJson["access_token"].(string)
	return accessToken, nil
}

func GetIdByClientId(clientId string, secret *corev1.Secret) (string, error) {
	token, err := GetHyperauthAdminToken(secret)
	if err != nil {
		return "", err
	}

	url := SetSecureHyperAuthURL(KEYCLOAK_ADMIN_SERVICE_GET_CLIENTS, nil)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	reqDump, _ := httputil.DumpRequest(req, true)
	if !IsOK(resp.StatusCode) {
		return "", fmt.Errorf("failed to get client: " + string(reqDump) + "\n" + resp.Status)
	}

	respJson := &[]ClientConfig{}
	err = json.NewDecoder(resp.Body).Decode(respJson)
	if err != nil {
		return "", err
	}

	for _, data := range *respJson {
		if data.ClientId == clientId {
			return data.Id, nil
		}
	}

	return "", nil
}

func CreateClient(clientConfig ClientConfig, secret *corev1.Secret) error {
	token, err := GetHyperauthAdminToken(secret)
	if err != nil {
		return err
	}

	id, err := GetIdByClientId(clientConfig.ClientId, secret)
	if err != nil {
		return err
	}
	if IsClientExist(id) {
		return nil
	}

	// data := ClientConfig{
	// 	ClientId:                  clientConfig.ClientId,
	// 	Secret:                    "tmax-client-secret",
	// 	DirectAccessGrantsEnabled: true,
	// 	RedirectUris:              []string{"*"},
	// 	// ServiceAccountsEnabled:    true,
	// }
	jsonData, err := json.Marshal(clientConfig)
	if err != nil {
		return err
	}
	playload := bytes.NewBuffer(jsonData)

	url := SetSecureHyperAuthURL(KEYCLOAK_ADMIN_SERVICE_CREATE_CLIENT, nil)
	req, err := http.NewRequest(http.MethodPost, url, playload)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !IsOK(resp.StatusCode) {
		reqDump, _ := httputil.DumpRequest(req, true)
		return fmt.Errorf("failed to create client: " + string(reqDump) + "\n" + resp.Status)
	}

	return nil
}

// func UpdateClient(clientId string, secret *corev1.Secret) error {
// 	token, err := GetHyperauthAdminToken(secret)
// 	if err != nil {
// 		return err
// 	}

// 	id, err := GetClients(clientId, secret)
// 	if err != nil {
// 		return err
// 	}
// 	if !IsClientExist(id) {
// 		return fmt.Errorf("client not found")
// 	}

// 	data := ClientConfig{
// 		ImplicitFlowEnabled: true,
// 	}
// 	jsonData, err := json.Marshal(data)
// 	if err != nil {
// 		return err
// 	}
// 	playload := bytes.NewBuffer(jsonData)

// 	params := map[string]string{
// 		"id": id,
// 	}
// 	url := SetSecureHyperAuthURL(KEYCLOAK_ADMIN_SERVICE_UPDATE_CLIENT, params)
// 	req, err := http.NewRequest(http.MethodPost, url, playload)
// 	if err != nil {
// 		return err
// 	}
// 	req.Header.Add("Content-Type", "application/json")
// 	req.Header.Add("Authorization", "Bearer "+token)

// 	client := &http.Client{}
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Body.Close()

// 	if !IsOK(resp.StatusCode) {
// 		reqDump, _ := httputil.DumpRequest(req, true)
// 		return fmt.Errorf("failed to create client-level role: " + string(reqDump) + "\n" + resp.Status)
// 	}

// 	return nil
// }

func CreateClientLevelProtocolMapper(clientId string, mapperName string, secret *corev1.Secret) error {
	token, err := GetHyperauthAdminToken(secret)
	if err != nil {
		return err
	}

	id, err := GetIdByClientId(clientId, secret)
	if err != nil {
		return err
	}
	if !IsClientExist(id) {
		return fmt.Errorf("client not found")
	}

	data := ProtocolMapperConfig{
		Name:           mapperName,
		Protocol:       "openid-connect",
		ProtocolMapper: "oidc-audience-mapper",
		Config: MapperConfig{
			IncludedClientAudience: clientId,
			IdTokenClaim:           false,
			AccessTokenClaim:       true,
			UserInfoTokenClaim:     false,
		},
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	playload := bytes.NewBuffer(jsonData)

	params := map[string]string{
		"id": id,
	}
	url := SetSecureHyperAuthURL(KEYCLOAK_ADMIN_SERVICE_CREATE_CLIENT_PROTOCOL_MAPPERS, params)
	req, err := http.NewRequest(http.MethodPost, url, playload)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !IsOK(resp.StatusCode) {
		reqDump, _ := httputil.DumpRequest(req, true)
		return fmt.Errorf("failed to create protocol mapper: " + string(reqDump) + "\n" + resp.Status)
	}

	return nil
}

func CreateClientLevelRole(clientId string, roleName string, secret *corev1.Secret) error {
	token, err := GetHyperauthAdminToken(secret)
	if err != nil {
		return err
	}

	id, err := GetIdByClientId(clientId, secret)
	if err != nil {
		return err
	}
	if !IsClientExist(id) {
		return fmt.Errorf("client not found")
	}

	data := RoleConfig{
		Name: roleName,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	playload := bytes.NewBuffer(jsonData)

	params := map[string]string{
		"id": id,
	}
	url := SetSecureHyperAuthURL(KEYCLOAK_ADMIN_SERVICE_CREATE_ROLES, params)
	req, err := http.NewRequest(http.MethodPost, url, playload)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !IsOK(resp.StatusCode) {
		reqDump, _ := httputil.DumpRequest(req, true)
		return fmt.Errorf("failed to create client-level role: " + string(reqDump) + "\n" + resp.Status)
	}

	return nil
}

func GetUserIdByEmail(userEmail string, secret *corev1.Secret) (string, error) {
	token, err := GetHyperauthAdminToken(secret)
	if err != nil {
		return "", err
	}

	params := map[string]string{
		"userEmail": userEmail,
		"token":     token,
	}
	url := SetSecureHyperAuthURL(HYPERAUTH_SERVICE_GET_USER_ID_BY_EMAIL, params)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	reqDump, _ := httputil.DumpRequest(req, true)
	if !IsOK(resp.StatusCode) {
		return "", fmt.Errorf("failed to get user: " + string(reqDump) + "\n" + resp.Status)
	}

	respJson := &UserConfig{}
	err = json.NewDecoder(resp.Body).Decode(respJson)
	if err != nil {
		return "", err
	}
	if respJson == nil {
		return "", fmt.Errorf("user not found")
	}

	return respJson.Id, nil
}

func GetRoleIdByRoleName(clientId string, roleName string, secret *corev1.Secret) (string, error) {
	token, err := GetHyperauthAdminToken(secret)
	if err != nil {
		return "", err
	}

	id, err := GetIdByClientId(clientId, secret)
	if err != nil {
		return "", err
	}
	if !IsClientExist(id) {
		return "", fmt.Errorf("client not found")
	}

	params := map[string]string{
		"id":       id,
		"roleName": roleName,
	}
	url := SetSecureHyperAuthURL(KEYCLOAK_ADMIN_SERVICE_GET_ROLE_BY_NAME, params)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	reqDump, _ := httputil.DumpRequest(req, true)
	if !IsOK(resp.StatusCode) {
		return "", fmt.Errorf("failed to get role: " + string(reqDump) + "\n" + resp.Status)
	}

	respJson := &RoleConfig{}
	err = json.NewDecoder(resp.Body).Decode(respJson)
	if err != nil {
		return "", err
	}
	if respJson == nil {
		return "", fmt.Errorf("role not found")
	}

	return respJson.Id, nil
}

func AddClientLevelRolesToUserRoleMapping(clientId string, roleName string, userEmail string, secret *corev1.Secret) error {
	token, err := GetHyperauthAdminToken(secret)
	if err != nil {
		return err
	}

	id, err := GetIdByClientId(clientId, secret)
	if err != nil {
		return err
	}
	if !IsClientExist(id) {
		return fmt.Errorf("client not found")
	}

	userId, err := GetUserIdByEmail(userEmail, secret)
	if err != nil {
		return err
	}

	roleId, err := GetRoleIdByRoleName(clientId, roleName, secret)
	if err != nil {
		return err
	}
	data := []RoleConfig{
		{
			Id:   roleId,
			Name: roleName,
		},
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	playload := bytes.NewBuffer(jsonData)

	params := map[string]string{
		"userId": userId,
		"id":     id,
	}
	url := SetSecureHyperAuthURL(KEYCLOAK_ADMIN_SERVICE_ADD_ROLE_TO_USER, params)
	req, err := http.NewRequest(http.MethodPost, url, playload)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !IsOK(resp.StatusCode) {
		reqDump, _ := httputil.DumpRequest(req, true)
		return fmt.Errorf("failed to add role to user: " + string(reqDump) + "\n" + resp.Status)
	}

	return nil
}

func GetClientScopesIdByName(name string, secret *corev1.Secret) (string, error) {
	token, err := GetHyperauthAdminToken(secret)
	if err != nil {
		return "", err
	}

	url := SetSecureHyperAuthURL(KEYCLOAK_ADMIN_SERVICE_GET_CLIENT_SCOPES, nil)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if !IsOK(resp.StatusCode) {
		reqDump, _ := httputil.DumpRequest(req, true)
		return "", fmt.Errorf("failed to get client scope id: " + string(reqDump) + "\n" + resp.Status)
	}

	respJson := &[]ClientScopeConfig{}
	err = json.NewDecoder(resp.Body).Decode(respJson)
	if err != nil {
		return "", err
	}

	for _, data := range *respJson {
		if data.Name == name {
			return data.Id, nil
		}
	}

	return "", fmt.Errorf("client scope not found")
}

func AddClientScopeToClient(clientId string, clientScopeName string, secret *corev1.Secret) error {
	token, err := GetHyperauthAdminToken(secret)
	if err != nil {
		return err
	}

	id, err := GetIdByClientId(clientId, secret)
	if err != nil {
		return err
	}
	if !IsClientExist(id) {
		return nil
	}

	clientScopeId, err := GetClientScopesIdByName(clientScopeName, secret)
	if err != nil {
		return err
	}

	params := map[string]string{
		"id":            id,
		"clientScopeId": clientScopeId,
	}
	url := SetSecureHyperAuthURL(KEYCLOAK_ADMIN_SERVICE_ADD_CLIENT_SCOPE_TO_CLIENT, params)
	req, err := http.NewRequest(http.MethodPut, url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !IsOK(resp.StatusCode) {
		reqDump, _ := httputil.DumpRequest(req, true)
		return fmt.Errorf("failed to add client scope to client: " + string(reqDump) + "\n" + resp.Status)
	}

	return nil
}

func DeleteClient(clientId string, secret *corev1.Secret) error {
	token, err := GetHyperauthAdminToken(secret)
	if err != nil {
		return err
	}

	id, err := GetIdByClientId(clientId, secret)
	if err != nil {
		return err
	}
	if !IsClientExist(id) {
		return nil
	}

	params := map[string]string{
		"id": id,
	}
	url := SetSecureHyperAuthURL(KEYCLOAK_ADMIN_SERVICE_DELETE_CLIENT, params)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !IsOK(resp.StatusCode) {
		reqDump, _ := httputil.DumpRequest(req, true)
		return fmt.Errorf("failed to delete client: " + string(reqDump) + "\n" + resp.Status)
	}

	return nil
}

func IsOK(check int) bool {
	SuccessStatusList := map[int]bool{
		http.StatusOK:             true,
		http.StatusCreated:        true,
		http.StatusNoContent:      true,
		http.StatusPartialContent: true,
		http.StatusConflict:       true,
		// http.StatusContinue:       true,
	}
	_, ok := SuccessStatusList[check]
	return ok
}

func IsClientExist(id string) bool {
	return id != ""
}

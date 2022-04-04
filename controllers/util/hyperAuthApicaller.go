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
	ClientId                  string   `json:"clientId,omitempty"`
	Id                        string   `json:"id,omitempty"`
	Secret                    string   `json:"secret,omitempty"`
	DirectAccessGrantsEnabled bool     `json:"directAccessGrantsEnabled,omitempty"`
	RedirectUris              []string `json:"redirectUris,omitempty"`
}

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

// type ClientLevelRole struct {
// 	Id          string            `json:"id"`
// 	Name        string            `json:"name"`
// 	Composite   bool              `json:"composite"`
// 	ClientRole  bool              `json:"clientRole"`
// 	ContainerId string            `json:"containerId"`
// 	Attributes  map[string]string `json:"attributes"`
// 	// TestAttr    string            `json:"testAttr,omitempty"`
// }

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

func GetHyperauthAdminToken(secret corev1.Secret) (string, error) {
	// Make Body for Content-Type (application/x-www-form-urlencoded)
	id, password := string(secret.Data["HYPERAUTH_ADMIN"]), string(secret.Data["HYPERAUTH_PASSWORD"])
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", id)
	data.Set("password", password)
	data.Set("client_id", "admin-cli")

	// Make Request Object
	url := SetSecureHyperAuthURL(HYPERAUTH_SERVICE_NAME_GET_ADMIN_TOKEN, nil)
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

func GetClients(clientId string, token string) (string, error) {
	url := SetSecureHyperAuthURL(HYPERAUTH_SERVICE_NAME_GET_CLIENT, nil)
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

func CreateClient(clientId string, token string) error {
	id, err := GetClients(clientId, token)
	if err != nil {
		return err
	}
	if id != "" {
		return nil
	}

	data := ClientConfig{
		ClientId:                  clientId,
		Secret:                    "tmax-client-secret",
		DirectAccessGrantsEnabled: true,
		RedirectUris:              []string{"*"},
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	playload := bytes.NewBuffer(jsonData)

	url := SetSecureHyperAuthURL(HYPERAUTH_SERVICE_NAME_CREATE_CLIENT, nil)
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

func CreateProtocolMapper(clientId string, token string) error {
	id, err := GetClients(clientId, token)
	if err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("client not found")
	}

	data := ProtocolMapperConfig{
		Name:           "kibana",
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
	url := SetSecureHyperAuthURL(HYPERAUTH_SERVICE_NAME_CREATE_MAPPER, params)
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

// func CreateRoleForSingleCluster(clientId string, token string) error {
// 	data := PlayLoad{
// 		RoleName: KIBANA_ROLE_NAME,
// 		Id:       KIBANA_ROLE_NAME,
// 	}
// 	jsonData, err := json.Marshal(data)
// 	if err != nil {
// 		return err
// 	}
// 	playload := bytes.NewBuffer(jsonData)

// 	params := map[string]string{
// 		"id": clientId,
// 	}
// 	url := SetSecureHyperAuthURL(HYPERAUTH_SERVICE_NAME_CREATE_ROLES, params)
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
// 		return fmt.Errorf("failed to create role for client [" + clientId + "]: " + string(reqDump) + "\n" + resp.Status)
// 	}

// 	return nil
// }

// func GetHyperauthClientLevelRole(clientId string, roleName string, token string) (*ClientLevelRole, error) {
// 	// data := RoleConfig{
// 	// 	Name: KIBANA_ROLE_NAME,
// 	// 	Id:   KIBANA_ROLE_NAME,
// 	// }
// 	// jsonData, err := json.Marshal(data)
// 	// if err != nil {
// 	// 	return err
// 	// }
// 	// playload := bytes.NewBuffer(jsonData)

// 	params := map[string]string{
// 		"id":       clientId,
// 		"roleName": roleName,
// 	}
// 	url := SetSecureHyperAuthURL(HYPERAUTH_SERVICE_NAME_GET_ROLE, params)
// 	req, err := http.NewRequest(http.MethodGet, url, nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	req.Header.Add("Authorization", "Bearer "+token)

// 	client := &http.Client{}
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()

// 	if !IsOK(resp.StatusCode) {
// 		reqDump, _ := httputil.DumpRequest(req, true)
// 		return nil, fmt.Errorf("failed to create role for client [" + clientId + "]: " + string(reqDump) + "\n" + resp.Status)
// 	}

// 	respJson := &ClientLevelRole{}
// 	err = json.NewDecoder(resp.Body).Decode(respJson)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return respJson, nil
// }

// func UpdateUserForSingleCluster(userId string, token string) error {
// 	data := []PlayLoad{
// 		{
// 			RoleName: KIBANA_ROLE_NAME,
// 			Id:       KIBANA_ROLE_NAME,
// 		},
// 	}
// 	jsonData, err := json.Marshal(data)
// 	if err != nil {
// 		return err
// 	}
// 	playload := bytes.NewBuffer(jsonData)

// 	params := map[string]string{
// 		"id": clientId,
// 	}
// 	url := SetSecureHyperAuthURL(HYPERAUTH_SERVICE_NAME_UPDATE_USER, params)
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
// 		return fmt.Errorf("failed to update user for client [" + clientId + "]: " + resp.Status)
// 	}

// 	return nil
// }

func DeleteClient(clientId string, token string) error {
	id, err := GetClients(clientId, token)
	if err != nil {
		return err
	}
	if id == "" {
		return nil
	}

	params := map[string]string{
		"id": id,
	}
	url := SetSecureHyperAuthURL(HYPERAUTH_SERVICE_NAME_DELETE_CLIENT, params)
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
		// http.StatusConflict:       true,
		// http.StatusContinue:       true,
	}
	_, ok := SuccessStatusList[check]
	return ok
}

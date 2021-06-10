package util

import (
	"crypto/tls"
	"log"
	"net/http"
	"strings"
)

func Delete(namespace, cluster string) error {
	// hypercloud api call
	url := "https://hypercloud5-api-server-service.hypercloud5-system.svc.cluster.local/namespaces/{namespace}/clustermanagers/{clustermanager}"

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	// inviteMail = strings.Replace(inviteMail, "@@LINK@@", bodyParameter["@@LINK@@"], -1)
	url = strings.Replace(url, "{namespace}", namespace, -1)
	url = strings.Replace(url, "{clustermanager}", cluster, -1)
	client := &http.Client{Transport: tr}
	_, err := client.Post(url, "application/json", nil)
	if err != nil {
		log.Fatalf("An Error Occured %v", err)
		return err
	}
	return nil
}

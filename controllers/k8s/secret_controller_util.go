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

package controllers

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	clusterV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"

	"github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

func CreateClusterRole(name string, targetGroup []string, verbList []string) *rbacv1.ClusterRole {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: targetGroup,
				Resources: []string{rbacv1.ResourceAll},
				Verbs:     verbList,
			},
			{
				APIGroups: []string{"apiregistration.k8s.io"},
				Resources: []string{rbacv1.ResourceAll},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	return clusterRole
}

func SADeleteList(adminSAName string) []types.NamespacedName {
	return []types.NamespacedName{
		{
			Name:      util.ArgoServiceAccount,
			Namespace: util.KubeNamespace,
		},
		{
			Name:      adminSAName,
			Namespace: util.KubeNamespace,
		},
	}
}

func SecretDeleteList(adminSAName string) []types.NamespacedName {
	return []types.NamespacedName{
		{
			Name:      util.ArgoServiceAccountTokenSecret,
			Namespace: util.KubeNamespace,
		},
		{
			Name:      adminSAName + "-token",
			Namespace: util.KubeNamespace,
		},
	}
}

func CRBDeleteList(owner string, memberList []ClusterMemberInfo) []string {
	crbList := []string{
		"cluster-owner-crb-" + owner,
		"cluster-owner-sa-crb-" + owner,
		util.ArgoClusterRoleBinding,
	}
	for _, member := range memberList {
		if member.Status == "invited" && member.Attribute == "user" {
			// user 로 초대 된 member crb
			crbList = append(crbList, member.MemberId+"-user-rolebinding")
		} else if member.Status == "invited" && member.Attribute == "group" {
			// group 으로 초대 된 member crb
			crbList = append(crbList, member.MemberId+"-group-rolebinding")
		}
	}
	return crbList
}

func CRDeleteList() []string {
	return []string{
		"developer",
		"guest",
		util.ArgoClusterRole,
	}
}

func DeleteSAList(clientSet *kubernetes.Clientset, saList []types.NamespacedName) error {
	for _, targetSa := range saList {
		_, err := clientSet.
			CoreV1().
			ServiceAccounts(targetSa.Namespace).
			Get(context.TODO(), targetSa.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		} else {
			err := clientSet.
				CoreV1().
				ServiceAccounts(targetSa.Namespace).
				Delete(context.TODO(), targetSa.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func DeleteSecretList(clientSet *kubernetes.Clientset, secretList []types.NamespacedName) error {
	for _, targetSecret := range secretList {
		_, err := clientSet.
			CoreV1().
			Secrets(targetSecret.Namespace).
			Get(context.TODO(), targetSecret.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
		} else if err != nil {
			return err
		} else {
			err := clientSet.
				CoreV1().
				Secrets(targetSecret.Namespace).
				Delete(context.TODO(), targetSecret.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func DeleteCRBList(clientSet *kubernetes.Clientset, crbList []string) error {
	for _, targetCrb := range crbList {
		_, err := clientSet.
			RbacV1().
			ClusterRoleBindings().
			Get(context.TODO(), targetCrb, metav1.GetOptions{})
		if errors.IsNotFound(err) {
		} else if err != nil {
			return err
		} else {
			err := clientSet.
				RbacV1().
				ClusterRoleBindings().
				Delete(context.TODO(), targetCrb, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func DeleteCRList(clientSet *kubernetes.Clientset, crList []string) error {
	for _, targetCr := range crList {
		_, err := clientSet.
			RbacV1().
			ClusterRoles().
			Get(context.TODO(), targetCr, metav1.GetOptions{})
		if errors.IsNotFound(err) {
		} else if err != nil {
			return err
		} else {
			err := clientSet.
				RbacV1().
				ClusterRoles().
				Delete(context.TODO(), targetCr, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func GetAdminServiceAccountName(clusterManager clusterV1alpha1.ClusterManager) string {
	re, _ := regexp.Compile("[" + regexp.QuoteMeta(`!#$%&'"*+-/=?^_{|}~().,:;<>[]\`) + "`\\s" + "]")
	email := clusterManager.Annotations[util.AnnotationKeyOwner]
	adminServiceAccountName := re.ReplaceAllString(strings.Replace(email, "@", "-at-", -1), "-")
	return adminServiceAccountName
}

// Cluster member information
type ClusterMemberInfo struct {
	Id          int64     `json:"Id"`
	Namespace   string    `json:"Namespace"`
	Cluster     string    `json:"Cluster"`
	MemberId    string    `json:"MemberId"`
	Groups      []string  `json:"Groups"`
	MemberName  string    `json:"MemberName"`
	Attribute   string    `json:"Attribute"`
	Role        string    `json:"Role"`
	Status      string    `json:"Status"`
	CreatedTime time.Time `json:"CreatedTime"`
	UpdatedTime time.Time `json:"UpdatedTime"`
}

// db 로 부터 클러스터에 초대 된 member 들의 info 가져오기
func FetchMemberList(clusterManager clusterV1alpha1.ClusterManager) ([]ClusterMemberInfo, error) {
	jsonData, _ := util.List(clusterManager.Namespace, clusterManager.Name)
	memberList := []ClusterMemberInfo{}
	if err := json.Unmarshal(jsonData, &memberList); err != nil {
		return []ClusterMemberInfo{}, err
	}
	return memberList, nil
}

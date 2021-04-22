package util

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"k8s.io/klog"
)

const (
	DB_USER      = "postgres"
	DB_PASSWORD  = "tmax"
	DB_NAME      = "postgres"
	HOSTNAME     = "postgres-service.hypercloud5-system.svc"
	PORT         = 5432
	INSERT_QUERY = "INSERT INTO CLUSTER_MEMBER (namespace, cluster, member_id, member_name, attribute, role, status, createdTime, updatedTime) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)"
	DELETE_QUERY = "DELETE FROM CLUSTER_MEMBER WHERE cluster = $1"
)

var pg_con_info string

type ClusterMemberInfo struct {
	Id          int64
	Namespace   string
	Cluster     string
	MemberId    string
	MemberName  string
	Attribute   string
	Role        string
	Status      string
	CreatedTime time.Time
	UpdatedTime time.Time
}

func init() {
	pg_con_info = fmt.Sprintf("port=%d host=%s user=%s "+
		"password=%s dbname=%s sslmode=disable",
		PORT, HOSTNAME, DB_USER, DB_PASSWORD, DB_NAME)
}

func Delete(cluster string) error {
	db, err := sql.Open("postgres", pg_con_info)
	if err != nil {
		klog.Error(err)
		return err
	}
	defer db.Close()

	_, err = db.Exec(DELETE_QUERY, cluster)
	if err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

// func Insert(item *clusterv1alpha1.ClusterManager) error {
// 	db, err := sql.Open("postgres", pg_con_info)
// 	if err != nil {
// 		klog.Error(err)
// 		return err
// 	}
// 	defer db.Close()

// 	_, err = db.Exec(INSERT_QUERY, item.Namespace, item.Name, item.Annotations["owner"], item.MemberName, "user", "admin", "owner", time.Now(), time.Now())
// 	if err != nil {
// 		klog.Error(err)
// 		return err
// 	}

// 	return nil
// }
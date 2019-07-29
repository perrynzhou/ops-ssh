package db

import (
	"errors"
	log "logging"

	"github.com/boltdb/bolt"
)

const (
	DefaultClusterNodeBucket  = "CLUSTER_NODE"
	DefaultClusterGroupBucket = "CLUSTER_GROUP"
)
const (
	DefaultStorageFile = "./vsh.db"
	DefaultGroupKey    = "ClusterGroupKey"
)

var (
	HandleIsNilErr = errors.New("stroage handler is nil")
)
var DBHandler *bolt.DB

func InitDBHandler() error {
	if DBHandler == nil {
		db, err := bolt.Open(DefaultStorageFile, 0600, nil)
		if err != nil {
			return err
		}
		tx, err := db.Begin(true)
		if err != nil {
			return err
		}
		defer tx.Commit()
		log.Info("storage init success")

		if _, err = tx.CreateBucketIfNotExists([]byte(DefaultClusterNodeBucket)); err != nil {
			return err
		}
		if _, err = tx.CreateBucketIfNotExists([]byte(DefaultClusterGroupBucket)); err != nil {
			return err
		}
		DBHandler = db
	}
	return nil
}

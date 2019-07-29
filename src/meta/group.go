package meta

import (
	"db"
	"encode"
	"encoding/json"
	log "logging"
	"sync"

	"github.com/boltdb/bolt"
)

type Group struct {
	GroupMeta map[string]uint8    `json:"groups"`
	Ref       map[string][]string `json:"ref"` //key is group,value is ip
	Addrs     map[string]uint8    `json:"hosts"`
}

func FetchGroup() *Group {
	var group *Group
	if db.DBHandler == nil {
		return nil
	}
	err := db.DBHandler.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(db.DefaultClusterGroupBucket)).Get([]byte(db.DefaultGroupKey))
		if b == nil {
			log.Info("group just init", db.DefaultGroupKey)
			return nil
		}
		rb, err := encode.Decoding(b)
		if err != nil {
			return err
		}
		group = &Group{}
		if err := json.Unmarshal(rb, group); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Error(err)
		group = nil
		return nil
	}
	return group

}
func (g *Group) Bytes() []byte {
	b, err := json.Marshal(g)
	if err != nil {
		return nil
	}
	eb, err := encode.Encoding(b)
	if err != nil {
		return nil
	}
	return eb
}
func (group *Group) Update() error {
	mutex := &sync.Mutex{}
	mutex.Lock()
	defer mutex.Unlock()
	if db.DBHandler == nil {
		return db.HandleIsNilErr
	}
	err := db.DBHandler.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(db.DefaultClusterGroupBucket)).Put([]byte(db.DefaultGroupKey), group.Bytes())
	})
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}
func (group *Group) Delete() error {
	mutex := &sync.Mutex{}
	mutex.Lock()
	defer mutex.Unlock()
	if db.DBHandler == nil {
		return db.HandleIsNilErr
	}
	err := db.DBHandler.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(db.DefaultClusterGroupBucket)).DeleteBucket([]byte(db.DefaultClusterGroupBucket))
	})
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

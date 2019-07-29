package meta

import (
	"db"
	"encode"
	"encoding/json"
	"errors"
	"fmt"
	log "logging"
	"strings"

	"github.com/boltdb/bolt"
)

type Node struct {
	Ip        string `json:"ip"`
	Port      int    `json:"port"`
	UserName  string `json:"user"`
	Password  string `json:"password"`
	Tag       string `json:"tag,omitempty"`
	GroupName string `json:"group,omitempty"`
}

func FetchNode(ip string) *Node {
	var node *Node
	if db.DBHandler == nil {
		return nil
	}
	err := db.DBHandler.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(db.DefaultClusterNodeBucket)).Get([]byte(ip))
		if b == nil {
			return errors.New(fmt.Sprintf("node %s not exists", ip))
		}
		rb, err := encode.Decoding(b)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(rb, &node); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Warn("fetchNode ", ip, ":", err)
		node = nil
	}
	return node

}
func (n *Node) Compare(nd *Node) bool {
	if nd == nil {
		return false
	}
	if strings.Compare(n.Ip, nd.Ip) != 0 {
		return false
	}
	if n.Port != nd.Port {
		return false
	}
	if strings.Compare(n.UserName, nd.UserName) != 0 {
		return false
	}
	if strings.Compare(n.Password, nd.Password) != 0 {
		return false
	}
	if strings.Compare(n.GroupName, nd.GroupName) != 0 {
		return false
	}
	if strings.Compare(n.Tag, nd.Tag) != 0 {
		return false
	}

	return true
}
func (node *Node) Bytes() []byte {
	b, err := json.Marshal(node)
	if err != nil {
		return nil
	}
	rb, err := encode.Encoding(b)
	if err != nil {
		log.Error("node.Bytes:", err)
		return nil
	}
	return rb
}

func (node *Node) Update() error {
	if db.DBHandler == nil {
		return db.HandleIsNilErr
	}
	return db.DBHandler.Update(func(tx *bolt.Tx) error {
		if err := tx.Bucket([]byte(db.DefaultClusterNodeBucket)).Put([]byte(node.Ip), node.Bytes()); err != nil {
			log.Error(err)
			return err
		}
		return nil
	})
}
func (node *Node) Delete() error {
	if db.DBHandler == nil {
		return db.HandleIsNilErr
	}
	return db.DBHandler.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(db.DefaultClusterNodeBucket))
		b := bucket.Get([]byte(node.Ip))
		if b == nil {
			return errors.New(fmt.Sprintf("node ", node.Ip, " not exists"))
		}
		if err := bucket.Delete([]byte(node.Ip)); err != nil {
			log.Error(err)
			return err
		}
		return nil
	})
}

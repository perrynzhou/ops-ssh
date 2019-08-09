package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"meta"
	"os"
	"pb"
	"strings"
)

const (
	DefaultTemplateNodeSize = 5
)

type Cluster struct {
	PubUserName string       `json:"user"`
	PubPwd      string       `json:"password"`
	PubPort     int          `json:"port"`
	PubGroup    string       `json:"group,omitempty"`
	PubTag      string       `json:"tag,omitempty"`
	Nodes       []*meta.Node `json:"nodes,omitempty"`
}

func NewCluster(path string) (*Cluster, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cluster := &Cluster{}
	err = json.Unmarshal(b, cluster)
	if err != nil {
		return nil, err
	}
	for _, node := range cluster.Nodes {
		if len(node.Ip) == 0 || !ValidIpAddr(node.Ip) {
			continue
		}
		if node.Port == 0 {
			node.Port = cluster.PubPort
		}
		if len(node.Password) == 0 {
			node.Password = cluster.PubPwd
		}
		if len(node.UserName) == 0 {
			node.UserName = cluster.PubUserName
		}
		if len(node.GroupName) == 0 {
			node.GroupName = cluster.PubGroup
		}
		if len(node.Tag) == 0 {
			node.Tag = cluster.PubTag
		}

	}
	return cluster, nil
}
func (c *Cluster) InitRequest() (*pb.UpdateRequest, error) {

	nodeReq := &pb.UpdateRequest{
		PubGroup_: strings.ToLower(c.PubGroup),
		PubPwd:    c.PubPwd,

		PubUsername: c.PubUserName,
		PubPort:     int32(c.PubPort),
		PubTag:      strings.ToLower(c.PubTag),
		NodeMetas:   make([]*pb.NodeMeta, 0),
	}
	for _, v := range c.Nodes {
		meta := &pb.NodeMeta{
			Host:     v.Ip,
			Port:     int32(v.Port),
			Username: v.UserName,
			Password: v.Password,
			Tag:      strings.ToLower(v.Tag),
			Group:    strings.ToLower(v.GroupName),
		}
		nodeReq.NodeMetas = append(nodeReq.NodeMetas, meta)
	}
	return nodeReq, nil

}
func (c *Cluster) String() (string, error) {
	b, err := json.MarshalIndent(c, " ", " ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
func CreateTemplate(templateFile string) error {
	c := &Cluster{

		PubUserName: "root",
		PubPwd:      "test",
		PubPort:     22,
		PubGroup:    "image",
		PubTag:      "D1",
		Nodes:       make([]*meta.Node, 0),
	}
	for i := 0; i < DefaultTemplateNodeSize; i++ {
		node := &meta.Node{
			Ip: fmt.Sprintf("%s%d", "127.0.0.", i),
		}
		if c.PubPort != 0 {
			node.Port = int(c.PubPort)
		} else {
			node.Port = 22
		}
		if len(c.PubUserName) > 0 {
			node.UserName = c.PubUserName
		} else {
			node.UserName = "root"
		}

		if len(c.PubPwd) > 0 {
			node.Password = c.PubPwd
		} else {
			node.Password = "123456"
		}
		if len(c.PubTag) > 0 {
			node.Tag = strings.ToLower(c.PubTag)
		} else {
			node.Tag = strings.ToLower("D1")
		}
		c.Nodes = append(c.Nodes, node)
	}
	b, err := json.MarshalIndent(c, " ", " ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(templateFile, b, os.ModePerm)
}
func (c *Cluster) Dump(newPath string) error {
	b, err := json.MarshalIndent(c, " ", " ")
	if err != nil {
		return err
	}
	if err = ioutil.WriteFile(newPath, b, os.ModePerm); err != nil {
		return err
	}
	return nil
}

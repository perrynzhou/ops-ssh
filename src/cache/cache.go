package cache

import (
	"encode"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"meta"
	"os"
	"pb"
	"sort"
	"strings"
	"utils"
)

type Cache struct {
	GroupCache    map[string]int32      `json:"groups"`
	NodeCache     map[string]*meta.Node `json:"nodes"`
	GroupRefNodes map[string][]string   `json:"group_ref"`
}

func InitCache(c *Cache, res *pb.QueryResponse) error {
	if res == nil {
		return errors.New("response is nil")
	}
	if len(res.NodeMetas) == 0 || res.NodeMetas == nil {
		return errors.New("node is empty")
	}
	for k, v := range res.GroupMetas {
		c.GroupCache[k] = v

	}
	for _, n := range res.NodeMetas {
		node := &meta.Node{
			Ip:        n.Host,
			Port:      int(n.Port),
			UserName:  n.Username,
			Password:  n.Password,
			Tag:       n.Tag,
			GroupName: n.Group,
		}
		if c.GroupRefNodes[n.Group] == nil {
			c.GroupRefNodes[n.Group] = make([]string, 0)
		}
		if len(c.GroupRefNodes[n.Group]) == 0 {
			c.GroupRefNodes[n.Group] = append(c.GroupRefNodes[n.Group], n.Host)
		} else {
			addrs := strings.Join(c.GroupRefNodes[n.Group], ",")
			if !strings.Contains(addrs, n.Host) {
				c.GroupRefNodes[n.Group] = append(c.GroupRefNodes[n.Group], n.Host)
			}

		}
		if _, ok := c.NodeCache[n.Host]; !ok {
			c.NodeCache[n.Host] = node
		}
	}
	return nil
}

func (c *Cache) Encode() ([]byte, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	rb, err := encode.Encoding(b)
	if err != nil {
		return nil, err
	}
	return rb, nil
}
func (c *Cache) Decode(b []byte) error {
	rb, err := encode.Decoding(b)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(rb, c); err != nil {
		return nil
	}
	return nil
}
func (c *Cache) ReplaceCacheFile(path string) error {
	rootPath, _ := utils.Expand(fmt.Sprintf("~/%s", path))
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		if _, err = os.Create(rootPath); err != nil {
			fmt.Println("replaceCacheFile:", err)
			return err
		}
	}
	wb, err := c.Encode()
	if err != nil {
		return err
	}
	oldPath, _ := utils.Expand(fmt.Sprintf("~/%s", fmt.Sprintf(".%s.temp", path)))
	if err = ioutil.WriteFile(oldPath, wb, os.ModePerm); err != nil {
		return err
	}
	if err = os.Rename(oldPath, rootPath); err != nil {
		return err
	}

	return nil
}
func (c *Cache) Flush(path string) error {
	var rb []byte
	var err error
	if rb, err = json.Marshal(c); err != nil {
		return err
	}
	if rb, err = c.Encode(); err != nil {
		return err
	}
	return ioutil.WriteFile(path, rb, os.ModePerm)
}
func (c *Cache) OrderNode() []*meta.Node {
	nodes := make([]*meta.Node, 0)
	refNodes  := make(map[string][]*meta.Node)
	refKeys := make([]string,0)
	for _,node := range c.NodeCache {
		if refNodes[node.GroupName] == nil {
			refNodes[node.GroupName] = make([]*meta.Node,0)
			refKeys = append(refKeys,node.GroupName)
		}
		refNodes[node.GroupName] = append(refNodes[node.GroupName],node)
	}
	sort.Slice(refKeys, func(i, j int) bool {
		if strings.Compare(refKeys[i], refKeys[j]) < 0 {
			return true
		}
		return false
	})
    for _,key := range  refKeys {
    	rnodes := refNodes[key]
		sort.Slice(rnodes, func(i, j int) bool {
			if strings.Compare(rnodes[i].Ip, rnodes[j].Ip) < 0 {
				return true
			}
			return false
		})
		for _,node := range rnodes {
			nodes = append(nodes,node)
		}
	}
	return nodes
}
func (c *Cache) OrderGroup() []string {
	address := make([]string, 0)
	for _, node := range c.NodeCache {
		if len(address) == 0 {
			address = append(address, node.GroupName)
		} else {
			groups := strings.Join(address, ",")
			if !strings.Contains(groups, node.GroupName) {
				address = append(address, node.GroupName)
			}
		}
	}

	sort.Slice(address, func(i, j int) bool {
		if strings.Compare(address[i], address[j]) < 0 {
			return true
		}
		return false
	})
	return address
}

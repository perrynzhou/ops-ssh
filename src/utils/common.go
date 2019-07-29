package utils

import (
	"errors"
	"fmt"
	"meta"
	"net"
	"os/user"
	"path/filepath"
	"pb"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

func NewUpdateRequest(req *pb.UpdateRequest) []*meta.Node {
	nodes := make([]*meta.Node, 0)
	for _, v := range req.NodeMetas {
		node := &meta.Node{}
		node.Ip = v.Host
		node.Port = int(v.Port)
		node.UserName = v.Username
		node.Password = v.Password
		node.Tag = v.Tag
		node.GroupName = v.Group
		nodes = append(nodes, node)
	}
	return nodes
}
func ValidSshServer(node *meta.Node) error {
	sshConfig := &ssh.ClientConfig{
		User: node.UserName,
		Auth: []ssh.AuthMethod{
			ssh.Password(node.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second * 4,
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", node.Ip, node.Port), sshConfig)
	if err != nil {
		return err
	}
	defer client.Close()
	return nil
}
func ValidIpAddr(ipAddress string) bool {
	ipAddress = strings.Trim(ipAddress, " ")

	re, _ := regexp.Compile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
	if re.MatchString(ipAddress) {
		return true
	}
	return false
}

func Dir() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}
	if currentUser.HomeDir == "" {
		return "", errors.New("cannot find user-specific home dir")
	}

	return currentUser.HomeDir, nil
}

func Expand(path string) (string, error) {
	if len(path) == 0 {
		return path, nil
	}

	if path[0] != '~' {
		return path, nil
	}

	if len(path) > 1 && path[1] != '/' && path[1] != '\\' {
		return "", errors.New("cannot expand user-specific home dir")
	}

	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, path[1:]), nil
}
func GetUserName() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	ip, err := GetNetIp()
	if err != nil {
		return ip, err
	}
	return fmt.Sprintf("%s@%s", usr.Username, ip), nil
}
func GetNetIp() (string, error) {
	var ip string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ip, err
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
				break
			}

		}
	}
	return ip, nil
}

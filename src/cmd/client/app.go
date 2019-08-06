package main

import (
	"cache"
	"conn"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"meta"
	"os"
	"pb"
	"sort"
	"ssh"
	"strings"
	"text/tabwriter"
	"utils"
)

const (
	defaultTemplateFile            = "cluster_template.json"
	defaultClusterServerConfigFile = ".vsh_config.json"
	defaultCacheClusterFile        = ".vsh_cache.json"
	defaultClusterFile             = "cluster.json"
)

type Config struct {
	Addr string `json:"addr"`
	Port int    `json:"port"`
}

var formatWriter *tabwriter.Writer

func init() {
	if formatWriter == nil {
		formatWriter = new(tabwriter.Writer)
		formatWriter.Init(os.Stdout, 10, 4, 6, ' ', 0)
	}
}

func fetchCache(groupNames []string) (*cache.Cache, error) {
	var force bool
	cacheFile, _ := utils.Expand(fmt.Sprintf("~/%s", defaultCacheClusterFile))
	cli, err := initConn(defaultClusterServerConfigFile)
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	resp, err := cli.NewCacheSession()
	if err != nil {
		return nil, err
	}
	if resp.Response == 1 {
		force = true
	}
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		force = true
	}
	var c *cache.Cache
	if force {
		var res *pb.QueryResponse
		c = &cache.Cache{
			NodeCache:     make(map[string]*meta.Node),
			GroupRefNodes: make(map[string][]string),
			GroupCache:    make(map[string]int32),
		}
		if res, err = cli.NewViewSession(groupNames); err != nil {
			return nil, err
		}
		if err = cache.InitCache(c, res); err != nil {
			return nil, err
		}

		if err = c.ReplaceCacheFile(defaultCacheClusterFile); err != nil {
			return nil, err
		}
		return c, nil
	}
	b, err := ioutil.ReadFile(cacheFile)
	if err != nil {
		return nil, err
	}
	c = &cache.Cache{}
	if err = c.Decode(b); err != nil {
		return nil, err
	}
	return c, nil
}
func initConn(path string) (*conn.Conn, error) {
	rootPath, _ := utils.Expand(fmt.Sprintf("~/%s", path))
	var conf Config
	b, err := ioutil.ReadFile(rootPath)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(b, &conf); err != nil {
		return nil, err
	}
	return conn.NewConn(conf.Addr, conf.Port)

}
func usage() {
	fmt.Println("Usage:")
	fmt.Println("vsh [group|user|ip|template|decode|dump|load|delete]")
	fmt.Println("Available Commands:")
	fmt.Println("user      list current users")
	fmt.Println("group     list node group info")
	fmt.Println("decode    decode cluster info")
	fmt.Println("delete    delete nodes of group")
	fmt.Println("dump      dump cluster info")
	fmt.Println("load      load nodes")
	fmt.Println("template  create  cluster.json")
	fmt.Println("help      help for user")
}
func cmdHanleOneParameter(names []string) {
	var cmdName string
	var ip string
	if names == nil {
		cmdName = "node"
	} else {
		cmdName = strings.ToLower(names[0])
		if utils.ValidIpAddr(names[0]) {
			cmdName = "host"
			ip = names[0]
		} else {
			cmdName = strings.ToLower(names[0])
		}
	}
	if strings.Compare(cmdName, "dump") == 0 || strings.Compare(cmdName, "template") == 0 || strings.Compare(cmdName, "decode") == 0 || strings.Compare(cmdName, "node") == 0 || strings.Compare(cmdName, "group") == 0 || strings.Compare(cmdName, "user") == 0 || strings.Compare(cmdName, "host") == 0 {
		defer formatWriter.Flush()
		c, err := fetchCache(nil)
		if err != nil {
			fmt.Println("fetchCache :", err.Error())
			return
		}
		switch cmdName {
		case "node":
			fmt.Fprintln(formatWriter, "host\ttag\tgroup")
			if len(c.GroupRefNodes) == 0 {
				fmt.Println("empty nodes")
				return
			}
			nodes := c.OrderNode()
			for _, node := range nodes {
				fmt.Fprintf(formatWriter, "%s\t%s\t%s\n", node.Ip, node.Tag, node.GroupName)
			}
			return
		case "group":
			fmt.Fprintln(formatWriter, "nodes\tgroup")
			if len(c.GroupCache) == 0 {
				fmt.Println("enmpty group")
				return
			}

			groupNames := c.OrderGroup()
			for _, groupName := range groupNames {
				fmt.Fprintf(formatWriter, "%d\t%s\n", c.GroupCache[groupName], groupName)
			}

			break
		case "user":
			cli, err := initConn(defaultClusterServerConfigFile)
			if err != nil {
				fmt.Println("init conn:", err)
				return
			}
			resp, err := cli.NewUserSession()
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Fprintln(formatWriter, "user\tis_superuser")
			for userName, isSuperUser := range resp.Response {
				fmt.Fprintf(formatWriter, "%s\t%v\n", userName, isSuperUser)
			}
			break
		case "host":
			cli, err := initConn(defaultClusterServerConfigFile)
			if err != nil {
				fmt.Println("init conn:", err)
				return
			}
			defer cli.Close()
			_, err = cli.NewBasicSession()
			if err != nil {
				fmt.Println(err)
				return
			}

			cache, err := fetchCache(nil)
			if err != nil {
				fmt.Println("go host:", err)
				return
			}
			if _, ok := cache.NodeCache[ip]; !ok {
				fmt.Println("unknown node:", ip)
				return
			}
			node := cache.NodeCache[ip]
			if err = ssh.NewSSHConnection(node); err != nil {
				fmt.Printf("connect %s:%d:%v\n", node.Ip, node.Port, err)
				return
			}
			break
		case "template":
			cli, err := initConn(defaultClusterServerConfigFile)
			if err != nil {
				fmt.Println("init conn:", err)
				return
			}
			_, err = cli.NewBasicSession()
			if err != nil {
				fmt.Println(err)
				return
			}
			defer cli.Close()
			if _, err := os.Stat(defaultTemplateFile); os.IsNotExist(err) {
				utils.CreateTemplate(defaultTemplateFile)
				fmt.Println("create template success")
				return
			}
			fmt.Println("template exists in ", defaultTemplateFile)
			break
		case "dump":
			cli, err := initConn(defaultClusterServerConfigFile)
			if err != nil {
				fmt.Println("init conn:", err)
				return
			}
			_, err = cli.NewDumpSession()
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println("dump success on remote!")
			break
		}
	} else {
		usage()
	}
}
func cmdHanleTwoParameter(args []string) {
	if args == nil || len(args) != 2 {
		usage()
		return
	}
	cmdName := strings.ToLower(args[0])
	if strings.Compare(cmdName, "load") == 0 || strings.Compare(cmdName, "delete") == 0 {
		cli, err := initConn(defaultClusterServerConfigFile)
		if err != nil {
			fmt.Println("init connection:", err)
			return
		}
		cacheFile, err := utils.Expand(fmt.Sprintf("~/%s", defaultCacheClusterFile))
		if err != nil {
			fmt.Println("expand home dir:", err)
		}
		if _, err := os.Stat(cacheFile); err == nil {
			os.Remove(cacheFile)
		}
		switch cmdName {
		case "load":
			var resp *pb.UpdateResponse

			if _, err := os.Stat(args[1]); os.IsNotExist(err) {
				fmt.Println("load: ", args[1], " invalid")
				return
			}
			if resp, err = cli.NewUpdateSession(args[1]); err != nil {
				fmt.Println("new update session:", err)
				return
			}

			fmt.Fprintln(formatWriter, "host\tgroup\tmessage")
			if len(resp.Response) > 0 {
				sort.Slice(resp.Response, func(i, j int) bool {
					if strings.Compare(resp.Response[i].Addr, resp.Response[j].Addr) < 0 {
						return true
					}
					return false
				})
			}
			for _, res := range resp.Response {
				fmt.Fprintf(formatWriter, "%s\t%s\t%s\n", res.Addr, res.Group, res.Msg)
			}
			formatWriter.Flush()
			break
		case "delete":
			var resp *pb.DeleteResponse
			if resp, err = cli.NewDeleteSession(args[1:]); err != nil {
				fmt.Println("new update session:", err)
				return
			}
			fmt.Fprintln(formatWriter, "host\tgroup\tmessage")
			sort.Slice(resp.Response, func(i, j int) bool {
				if strings.Compare(resp.Response[i].Addr, resp.Response[j].Addr) < 0 {
					return true
				}
				return false

			})
			for _, res := range resp.Response {
				fmt.Fprintf(formatWriter, "%s\t%s\t%s\n", res.Addr, res.Group, res.Msg)
			}
			formatWriter.Flush()
			break
		}
	} else {
		usage()
	}
}
func main() {
	if len(os.Args) == 1 {
		cmdHanleOneParameter(nil)
	} else if len(os.Args) == 2 {
		if strings.Compare(strings.ToLower(os.Args[1]), "-h") == 0 || strings.Compare(strings.ToLower(os.Args[1]), "--help") == 0 {
			usage()
			return
		}
		cmdHanleOneParameter(os.Args[1:])
	} else if len(os.Args) == 3 {
		cmdHanleTwoParameter(os.Args[1:])
	} else {
		usage()
	}
}

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
	defaultClusterTemplateFile     = "template_cluster.json"
	defaultClusterServerConfigFile = ".vsh_config.json"
	defaultCacheClusterFile        = ".vsh_cache.json"
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
	fmt.Println("vsh [group|user|ip|template|dump|load|delete|{option_ip} run]")
	fmt.Println("Available Commands:")
	fmt.Println("user      list current users")
	fmt.Println("group     list node group info")
	fmt.Println("delete    delete nodes of group")
	fmt.Println("dump      dump cluster info")
	fmt.Println("load      load nodes")
	fmt.Println("run       execute shell command")
	fmt.Println("template  create  cluster.json")
	fmt.Println("help      help for user")
}
func hanleOneCmd(names []string) {
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
	defer formatWriter.Flush()
	cli, err := initConn(defaultClusterServerConfigFile)
	if err != nil {
		fmt.Println("init conn:", err)
		return
	}
	_, err = cli.NewBasicSession()
	if err != nil {
		fmt.Println(err)
		os.Remove(defaultCacheClusterFile)
		return
	}
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
		resp, err := cli.NewUserSession()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Fprintln(formatWriter, "user\ttype")
		for userName, userType := range resp.Response {
			fmt.Fprintf(formatWriter, "%s\t%v\n", userName, userType)
		}
		break
	case "host":
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
		if _, err := os.Stat(defaultClusterTemplateFile); os.IsNotExist(err) {
			utils.CreateTemplate(defaultClusterTemplateFile)
			fmt.Println("create template success")
			return
		}
		fmt.Println("template exists in ", defaultClusterTemplateFile)
		break
	case "dump":
		_, err = cli.NewDumpSession()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("dump success on remote!")
		break
	default:
		usage()
		break
	}

}

func hanleMultiCmd(args []string) {
	if args == nil || len(args) <= 1 {
		usage()
		return
	}
	cmdName := strings.ToLower(args[0])
	cli, err := initConn(defaultClusterServerConfigFile)
	if err != nil {
		fmt.Println("init connection:", err)
		return
	}
	_, err = cli.NewBasicSession()
	if err != nil {
		fmt.Println(err)
		os.Remove(defaultCacheClusterFile)
		return
	}
	if strings.Compare(cmdName, "load") == 0 || strings.Compare(cmdName, "delete") == 0 {

		cacheFile, err := utils.Expand(fmt.Sprintf("~/%s", defaultCacheClusterFile))
		if err != nil {
			fmt.Println("expand home dir:", err)
		}
		if _, err := os.Stat(cacheFile); err == nil {
			os.Remove(cacheFile)
		}
	}
	switch cmdName {
	case "run":
		cmdSize := len(args) - 1
		cmds := make([]string, cmdSize)
		for i := 0; i < cmdSize; i++ {
			cmds[i] = strings.ToLower(args[i+1])
		}
		c, err := fetchCache(nil)
		if err != nil {
			fmt.Println("fetchCache :", err.Error())
			return
		}
		nodes := c.OrderNode()
		exeCmd := strings.Join(cmds, " ")
		for _, node := range nodes {
			output, err := ssh.Run(node, exeCmd)
			fmt.Printf("********************%s***************************\n", node.Ip)
			fmt.Printf("%s $ %s\n", node.Ip, exeCmd)
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println(string(output))
			}
		}
		break
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
	default:
		usage()
		break
	}
}
func main() {
	if len(os.Args) == 1 {
		hanleOneCmd(nil)
	} else if len(os.Args) == 2 {
		if strings.Compare(strings.ToLower(os.Args[1]), "-h") == 0 || strings.Compare(strings.ToLower(os.Args[1]), "--help") == 0 {
			usage()
		} else {
			hanleOneCmd(os.Args[1:])
		}
	} else {
		hanleMultiCmd(os.Args[1:])
	}
}

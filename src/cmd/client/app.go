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

	"github.com/spf13/cobra"
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

func main() {
	var cmdGo = &cobra.Command{
		Use:   "go [go to host]",
		Short: "go host",
		Long:  `ssh login host`,
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if args == nil || len(args) == 0 {
				fmt.Println("empty host")
				return
			}
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
			if _, ok := cache.NodeCache[args[0]]; !ok {
				fmt.Println("unknown node:", args[0])
				return
			}
			node := cache.NodeCache[args[0]]
			if err = ssh.NewSSHConnection(node); err != nil {
				fmt.Printf("connect %s:%d:%v\n", node.Ip, node.Port, err)
				return
			}

		},
	}
	var cmdList = &cobra.Command{
		Use:   "list [list node|group|user info]",
		Short: "list {node|group|user}",
		Long:  `list node,group,user info`,
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				args = []string{"node"}
			}
			arg1 := strings.ToLower(args[0])
			defer formatWriter.Flush()
			if strings.Compare(strings.ToLower("node"), arg1) == 0 || strings.Compare(strings.ToLower("group"), arg1) == 0 {
				c, err := fetchCache(nil)
				if err != nil {
					fmt.Println("list :", err.Error())
					return
				}

				switch arg1 {
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
				default:
					fmt.Errorf("unknown list type\n")
					return
				}

			} else if strings.Compare(strings.ToLower("user"), arg1) == 0 {
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

			} else {
				fmt.Errorf("list %s|%s|%s\n", "node", "group", "user")
			}
		},
	}
	var cmdGenTemplate = &cobra.Command{
		Use:   "template [create cluster template]",
		Short: "create  cluster.json ",
		Long:  `create template json file for cluster.json `,
		Args:  cobra.MaximumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
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
		},
	}
	var cmdDump = &cobra.Command{
		Use:   "dump [dump cluster info]",
		Short: "dump cluster info",
		Long:  `dump cluster info on remote server`,
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

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
			fmt.Println("decode cluster info success on remote service!")

		},
	}
	var cmdDecode = &cobra.Command{
		Use:   "decode [decode cluster info]",
		Short: "decode cluster info",
		Long:  `decode cluster encode file`,
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cli, err := initConn(defaultClusterServerConfigFile)
			if err != nil {
				fmt.Println("init conn:", err)
				return
			}
			resp, err := cli.NewDecodeSession()
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(resp.Message)

		},
	}
	var cmdUpdate = &cobra.Command{
		Use:   "load [load nodes to cluster]",
		Short: "load nodes",
		Long:  `load cluster nodes in storage`,
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			clusterConfigFile, err := utils.Expand(fmt.Sprintf("~/%s", defaultClusterFile))
			if err != nil {
				fmt.Println("load cluster:", err.Error())
				return
			}
			if args != nil && len(args) > 0 {
				clusterConfigFile = args[0]
			}
			cli, err := initConn(defaultClusterServerConfigFile)
			if err != nil {
				fmt.Println("init conn:", err)
				return
			}

			var resp *pb.UpdateResponse

			if _, err := os.Stat(clusterConfigFile); os.IsNotExist(err) {
				fmt.Println("load: cluster file invalid")
				return
			}
			if resp, err = cli.NewUpdateSession(clusterConfigFile); err != nil {
				fmt.Println("new update session:", err)
				return
			}
			cacheFile, _ := utils.Expand(fmt.Sprintf("~/%s", defaultCacheClusterFile))
			if _, err := os.Stat(cacheFile); err == nil {
				os.Remove(cacheFile)
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

		},
	}
	var cmdDelete = &cobra.Command{
		Use:   "delete [delete group from cluster]",
		Short: "delete nodes of group",
		Long:  `delete group nodes in storage`,
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cli, err := initConn(defaultClusterServerConfigFile)
			if err != nil {
				fmt.Println("init conn:", err)
				return
			}
			var resp *pb.DeleteResponse
			if resp, err = cli.NewDeleteSession(args); err != nil {
				fmt.Println("new update session:", err)
				return
			}
			cacheFile, err := utils.Expand(fmt.Sprintf("~/%s", defaultCacheClusterFile))
			if _, err := os.Stat(cacheFile); err == nil {
				os.Remove(cacheFile)
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
		},
	}
	var rootCmd = &cobra.Command{Use: "vsh"}
	rootCmd.AddCommand(cmdGenTemplate)
	rootCmd.AddCommand(cmdUpdate)
	rootCmd.AddCommand(cmdDelete)
	rootCmd.AddCommand(cmdGo)
	rootCmd.AddCommand(cmdList)
	rootCmd.AddCommand(cmdDecode)
	rootCmd.AddCommand(cmdDump)
	rootCmd.Execute()

}

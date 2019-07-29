package server

import (
	"db"
	"encode"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	log "logging"
	"meta"
	"net"
	"os"
	"pb"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"utils"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	encodeDumpFile             = "cluster_dump.json"
	decodeDumpFile             = "decode_cluster_dump.json"
	defaultAuthorityConfigFile = "template_authority_config.json"
)

type UserRefNode struct {
	Name    string   `json:"uname"`
	Type    int      `json:"type"`
	Address []string `json:"addresses"`
}

type AuthorityConfig struct {
	PublicAddress []string      `json:"pub_nodes"`
	UserRefNodes  []UserRefNode `json:"user_ref_nodes"`
}

type UserInfo struct {
	Type             int
	IsNeedUpateCache bool
}
type Server struct {
	port                int
	stop                chan struct{}
	wg                  *sync.WaitGroup
	userPrivilege       map[string]*UserInfo //key is Name,value is privileges
	accessNode          map[string][]string  //key is Name,value is iplist
	mutex               *sync.Mutex
	dumpMutex           *sync.Mutex
	timeOut             time.Duration
	authorityConfigPath string
}

func NewAuthorityConfig(path string) (*AuthorityConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	b, err := ioutil.ReadFile(path)

	if err != nil {

		return nil, err
	}
	authorityConfig := &AuthorityConfig{
		PublicAddress: make([]string, 0),
		UserRefNodes:  make([]UserRefNode, 0),
	}
	if err = json.Unmarshal(b, authorityConfig); err != nil {
		return nil, err
	}

	for index := 0; index < len(authorityConfig.UserRefNodes); index++ {
		if authorityConfig.UserRefNodes[index].Type > 1 && authorityConfig.UserRefNodes[index].Type < 0 {
			authorityConfig.UserRefNodes[index].Type = 0 //default is normal user
		}
	}

	return authorityConfig, nil
}
func NewServer(port, timeSeconds int, configPath string, wg *sync.WaitGroup) *Server {
	if err := db.InitDBHandler(); err != nil {
		log.Fatal(err)
	}

	server := &Server{
		port:                port,
		wg:                  wg,
		stop:                make(chan struct{}),
		mutex:               &sync.Mutex{},
		dumpMutex:           &sync.Mutex{},
		accessNode:          make(map[string][]string),
		userPrivilege:       make(map[string]*UserInfo),
		timeOut:             time.Duration(timeSeconds) * time.Minute,
		authorityConfigPath: configPath,
	}
	if err := initServerAuthorityConfig(configPath, false, server); err != nil {
		log.Error("initServerAuthorityConfig:", err)
		return nil
	}
	log.Info("server:", server)
	return server
}
func initServerAuthorityConfig(configPath string, isDelKeys bool, s *Server) error {
	authorityConfig, err := NewAuthorityConfig(configPath)
	if err != nil {
		return err
	}
	if authorityConfig == nil {
		return errors.New("authorityConfig is nil")
	}
	if authorityConfig.UserRefNodes == nil || len(authorityConfig.UserRefNodes) == 0 {
		return errors.New("userrefnodes is nil or empty")
	}
	log.Info("---authorityConfig:", authorityConfig)

	s.mutex.Lock()
	defer s.mutex.Unlock()
	accessNode := make(map[string][]string)
	userPrivilege := make(map[string]*UserInfo)
	for _, userRefNode := range authorityConfig.UserRefNodes {
		address := make(map[string]uint8)
		if accessNode[userRefNode.Name] == nil {
			accessNode[userRefNode.Name] = make([]string, 0)
		}
		for _, ip := range authorityConfig.PublicAddress {
			if _, ok := address[ip]; !ok {
				accessNode[userRefNode.Name] = append(accessNode[userRefNode.Name], ip)
				address[ip] = 1
			}
		}
		if userRefNode.Type == 0 {
			for _, ip := range userRefNode.Address {
				if _, ok := address[ip]; !ok {
					accessNode[userRefNode.Name] = append(accessNode[userRefNode.Name], ip)
					address[ip] = 1
				}
			}
		}
		log.Info(userRefNode.Name, " access ", accessNode[userRefNode.Name])
		if _, ok := userPrivilege[userRefNode.Name]; !ok {
			userPrivilege[userRefNode.Name] = &UserInfo{
				Type:             userRefNode.Type,
				IsNeedUpateCache: isDelKeys,
			}
			log.Info("register user :", userRefNode.Name, ",info:", userPrivilege[userRefNode.Name])
		}
	}
	if isDelKeys {
		for k, _ := range s.accessNode {
			delete(s.accessNode, k)
		}
		for k, _ := range s.userPrivilege {
			delete(s.userPrivilege, k)
		}
	}
	s.accessNode = accessNode
	s.userPrivilege = userPrivilege
	return nil
}

func (s *Server) reloadAuthorityConfig(done chan struct{}) {
	watch, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error("NewWatcher:", err)
		return
	}
	defer watch.Close()

	go func() {
		for {
			select {
			case ev := <-watch.Events:
				log.Info(ev.Name, " change!!!")
				log.Info("event op:", ev.Op, ",ev string:", ev.String())
				if err := initServerAuthorityConfig(s.authorityConfigPath, true, s); err != nil {
					log.Error("initServerAuthorityConfig:", err)
				} else {
					for username, userInfo := range s.userPrivilege {
						userInfo.IsNeedUpateCache = true
						log.Info(username, " need to update cache:", true)
					}
					log.Info("reload ", s.authorityConfigPath, " success!")
					log.Info("authority info:", s.userPrivilege)
					log.Info("access nodes:", s.accessNode)
				}
				err = watch.Add(s.authorityConfigPath)
				if err != nil {
					log.Error("watch.Add:", err)
				}
			case err := <-watch.Errors:
				log.Info("error : ", err)
			case <-done:
				log.Info("stop watch ", s.authorityConfigPath)
				return
			}
		}
	}()
	err = watch.Add(s.authorityConfigPath)
	if err != nil {
		log.Error("watch.Add:", err)
		return
	}
	fmt.Println(" wait to stop watch file")
	<-done
}
func (s *Server) CreateTemplateAuthorityConfig() error {
	if _, err := os.Stat(defaultAuthorityConfigFile); os.IsExist(err) {
		if err = os.Remove(defaultAuthorityConfigFile); err != nil {
			return err
		}
	}
	conf := &AuthorityConfig{
		PublicAddress: []string{"127.0.0.1", "127.0.0.2"},
		UserRefNodes:  make([]UserRefNode, 0),
	}
	count := 2
	uname, err := utils.GetUserName()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {

		userRefNode := UserRefNode{
			Name:    uname,
			Type:    0,
			Address: make([]string, 0),
		}

		for j := 0; j < count; j++ {
			ip := fmt.Sprintf("127.0.0.%d", j+1)
			userRefNode.Address = append(userRefNode.Address, ip)
		}
		conf.UserRefNodes = append(conf.UserRefNodes, userRefNode)
	}
	bin, err := json.MarshalIndent(conf, " ", " ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(defaultAuthorityConfigFile, bin, os.ModePerm)

}
func (s *Server) decodeDump(encodePath, decodePath string) error {
	b, err := ioutil.ReadFile(encodePath)
	if err != nil {
		return err
	}

	rb, err := encode.Decoding(b)
	if err != nil {
		return err
	}
	if err = ioutil.WriteFile(decodePath, rb, os.ModePerm); err != nil {
		return err
	}
	return nil
}
func (s *Server) checkAccessPermission(Name string) bool {
	defer log.Info("checkAccessPermission:", Name, ",userinfo:", s.userPrivilege[Name])
	if _, ok := s.userPrivilege[Name]; !ok {
		return false
	}
	return true
}
func (s *Server) checkSuperPermission(Name string) bool {
	defer log.Info("checkSuperPermission:", Name, ",userinfo:", s.userPrivilege[Name])
	if _, ok := s.userPrivilege[Name]; !ok {
		return false
	}
	if s.userPrivilege[Name].Type != 1 {
		return false
	}
	return true
}
func (s *Server) User(ctx context.Context, in *pb.UserRequest) (*pb.UserResponse, error) {
	for !s.checkAccessPermission(in.Username) {
		return nil, errors.New("Permission denied")
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	resp := &pb.UserResponse{
		Response: make(map[string]bool),
	}
	for username, info := range s.userPrivilege {
		if info.Type == 1 {
			resp.Response[username] = true
		} else {
			resp.Response[username] = false

		}
	}
	if len(resp.Response) == 0 {
		return nil, errors.New("empty user")
	}
	return resp, nil
}
func (s *Server) Decode(ctx context.Context, in *pb.DecodeRequest) (*pb.DecodeResponse, error) {
	resp := &pb.DecodeResponse{
		Response: -1,
	}
	for !s.checkSuperPermission(in.Username) {
		return resp, errors.New("Permission denied")
	}
	if err := s.decodeDump(encodeDumpFile, decodeDumpFile); err != nil {
		resp.Message = err.Error()
	} else {
		resp.Message = "decode success on remote!"
	}
	resp.Response = 0
	return resp, nil

}
func (s *Server) Dump(ctx context.Context, in *pb.DumpRequest) (*pb.DumpResponse, error) {
	resp := &pb.DumpResponse{
		Response: -1,
	}
	if len(in.Username) == 0 {
		return nil, errors.New("invalid Name request")
	}
	if !s.checkSuperPermission(in.Username) {
		return nil, errors.New("permission denied")
	}
	if err := s.internalDump(); err != nil {
		return nil, err
	}
	resp.Response = 0
	resp.Message = fmt.Sprintf("dump %s", encodeDumpFile, " success on server")
	return resp, nil
}
func (s *Server) Load(ctx context.Context, in *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	var err error

	if !s.checkAccessPermission(in.AuthorityUser) {
		return nil, errors.New("Permission denied")
	}
	nodes := utils.NewUpdateRequest(in)
	if nodes == nil || len(nodes) == 0 {
		return nil, errors.New("invalid nodes")
	}

	resp := &pb.UpdateResponse{
		Response: make([]*pb.Response, len(nodes)),
	}
	log.Info("got nodes len:", len(nodes))
	var group *meta.Group
	if group = meta.FetchGroup(); group == nil {
		group = &meta.Group{
			Ref:       make(map[string][]string),
			Addrs:     make(map[string]uint8),
			GroupMeta: make(map[string]uint8),
		}

	}
	groupRef := make(map[string]string)
	count := uint64(0)
	for index, node := range nodes {
		resp.Response[index] = &pb.Response{}
		resp.Response[index].Addr = node.Ip
		resp.Response[index].Group = node.GroupName

		if err := utils.ValidSshServer(node); err != nil {
			log.Error("valid ", node.Ip, ":", err)
			resp.Response[index].Msg = fmt.Sprint("failed")
		} else {

			curNode := meta.FetchNode(node.Ip)
			if !node.Compare(curNode) {
				log.Info("node ", node.Ip, " change")
				if err = node.Update(); err != nil {
					resp.Response[index].Msg = err.Error()
				} else {
					resp.Response[index].Msg = "success"
					atomic.AddUint64(&count, 1)
					if _, ok := groupRef[node.Ip]; !ok {
						groupRef[node.Ip] = node.GroupName
					}
				}
			} else {
				resp.Response[index].Msg = fmt.Sprintf("node %s exists", node.Ip)
			}
		}
	}

	if count > 0 {
		for _, userInfo := range s.userPrivilege {
			userInfo.IsNeedUpateCache = true
		}
	}
	for ipAddr, groupName := range groupRef {
		if group.Ref[groupName] == nil {
			group.Ref[groupName] = make([]string, 0)
			group.Ref[groupName] = append(group.Ref[groupName], ipAddr)
		}
		if len(group.Ref[groupName]) > 0 {
			groupAddrs := strings.Join(group.Ref[groupName], ",")
			if !strings.Contains(groupAddrs, ipAddr) {
				group.Ref[groupName] = append(group.Ref[groupName], ipAddr)
				log.Info("node:", ipAddr, " add to group ", groupName)
			}
		}
		if _, ok := group.GroupMeta[strings.ToLower(groupName)]; !ok {
			group.GroupMeta[strings.ToLower(groupName)] = uint8(1)
		}
		if _, ok := group.Addrs[ipAddr]; !ok {
			group.Addrs[ipAddr] = uint8(1)
		}
	}
	if len(groupRef) > 0 {
		if err := group.Update(); err != nil {
			log.Error(err)
			return nil, err
		}
	}
	return resp, err
}

func (s *Server) Delete(ctx context.Context, in *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if !s.checkSuperPermission(in.Username) {
		return nil, errors.New("Permission denied")
	}
	var group *meta.Group
	if group = meta.FetchGroup(); group == nil {
		return nil, errors.New("empty group")
	}
	log.Info("group info:", group)
	deleteResp := &pb.DeleteResponse{
		Response: make([]*pb.Response, 0),
	}
	delGroups := make([]string, 0)
	for _, groupName := range in.Groups {
		delGroups = append(delGroups, groupName)
	}
	if len(delGroups) == 0 {
		for k, _ := range group.GroupMeta {
			delGroups = append(delGroups, k)
		}
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	count := uint64(0)
	delNodes := make(map[string]uint8)
	for _, groupName := range delGroups {

		for _, addr := range group.Ref[groupName] {
			if _, ok := delNodes[addr]; ok {
				continue
			}
			atomic.AddUint64(&count, 1)
			node := &meta.Node{
				Ip: addr,
			}
			response := &pb.Response{
				Group: groupName,
				Addr:  node.Ip,
			}

			if err := node.Delete(); err != nil {
				response.Msg = "failed"
			} else {
				response.Msg = "success"
			}
			log.Info("delete node:", response)
			delete(group.Addrs, addr)
			deleteResp.Response = append(deleteResp.Response, response)
		}
		delete(group.GroupMeta, groupName)
		delete(group.Ref, groupName)
	}
	if count > 0 {
		for _, userInfo := range s.userPrivilege {
			userInfo.IsNeedUpateCache = true
		}
	}
	return deleteResp, nil
}

func (s *Server) Access(ctx context.Context, in *pb.BasicRequest) (*pb.BasicResponse, error) {
	if !s.checkAccessPermission(in.Username) {
		return nil, errors.New("Permission denied")
	}
	return &pb.BasicResponse{}, nil

}

func (s *Server) Cache(ctx context.Context, in *pb.CacheRequest) (*pb.CacheResponse, error) {
	if !s.checkAccessPermission(in.Username) {
		return nil, errors.New("Permission denied")
	}
	resp := &pb.CacheResponse{
		Response: 0,
	}
	if s.userPrivilege[in.Username].IsNeedUpateCache {
		resp.Response = 1
	}
	return resp, nil
}

func (s *Server) Query(ctx context.Context, in *pb.QueryRequest) (*pb.QueryResponse, error) {
	if !s.checkAccessPermission(in.Username) {
		return nil, errors.New("Permission denied")
	}
	groupInfo := meta.FetchGroup()
	if groupInfo == nil {
		return nil, errors.New("empty group")
	}
	res := &pb.QueryResponse{
		GroupMetas: make(map[string]int32),
		NodeMetas:  make([]*pb.NodeMeta, 0),
	}
	var queryGroups []string
	if in.GroupNames == nil || len(in.GroupNames) <= 0 {
		queryGroups = make([]string, 0)
		for groupName, isDel := range groupInfo.GroupMeta {
			if isDel == 1 {
				queryGroups = append(queryGroups, groupName)
			}
		}
	} else {
		queryGroups = in.GroupNames
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	currentHosts := make(map[string]uint8)
	for _, groupName := range queryGroups {
		hosts := groupInfo.Ref[strings.ToLower(groupName)]
		if len(hosts) <= 0 {
			return nil, errors.New("empty group")
		}
		for _, ip := range hosts {

			currentHosts[ip] = uint8(1)
		}

	}
	accessHosts := make([]string, 0)
	if s.checkSuperPermission(in.Username) {

		for ip, _ := range currentHosts {
			accessHosts = append(accessHosts, ip)
		}
	} else {
		for _, ip := range s.accessNode[in.Username] {
			if _, ok := currentHosts[ip]; ok {
				accessHosts = append(accessHosts, ip)
			} else {
				log.Info("node ", ip, " not exists in cluster")
			}
		}
	}
	if len(accessHosts) == 0 {
		return nil, errors.New("empty nodes")
	}
	log.Info("user:", in.Username, " can access:", strings.Join(accessHosts, ","))
	var nodeCount uint64
	for _, ip := range accessHosts {
		node := meta.FetchNode(ip)
		if node == nil {
			atomic.AddUint64(&nodeCount, 1)
			continue
		}
		log.Info("ip:", ip, "\ngroupMeta:", res.GroupMetas)
		res.GroupMetas[strings.ToLower(node.GroupName)] = res.GroupMetas[strings.ToLower(node.GroupName)] + 1

		nodeMeta := &pb.NodeMeta{
			Host:     node.Ip,
			Port:     int32(node.Port),
			Username: node.UserName,
			Password: node.Password,
			Tag:      node.Tag,
			Group:    node.GroupName,
		}
		log.Info("query node:", nodeMeta.Host, ",port:", nodeMeta.Port)
		res.NodeMetas = append(res.NodeMetas, nodeMeta)
	}
	if nodeCount == uint64(len(accessHosts)) {
		return nil, errors.New("empty nodes")
	}
	if s.userPrivilege[in.Username].IsNeedUpateCache {
		s.userPrivilege[in.Username].IsNeedUpateCache = false
	}
	return res, nil

}
func (s *Server) Stop() {
	s.stop <- struct{}{}
}
func (s *Server) fetchCluster() (*utils.Cluster, error) {
	var group *meta.Group
	if group = meta.FetchGroup(); group == nil {
		return nil, errors.New("group is nil")
	}
	c := &utils.Cluster{
		Nodes: make([]*meta.Node, 0),
	}
	for ip, _ := range group.Addrs {

		node := meta.FetchNode(ip)
		if node != nil {
			c.Nodes = append(c.Nodes, node)
		}
	}
	return c, nil
}
func (s *Server) internalDump() error {
	var newCluster *utils.Cluster
	var initFlag bool
	var count uint64
	c, err := s.fetchCluster()
	if err != nil {
		return err
	}
	if len(c.Nodes) == 0 {
		log.Info("current nodes size:", len(c.Nodes))
		return nil
	}
	fetchData := make(map[string]*meta.Node)
	localData := make(map[string]*meta.Node)
	s.dumpMutex.Lock()
	defer s.dumpMutex.Unlock()
	for _, node := range c.Nodes {
		fetchData[node.Ip] = node
	}
	if _, err := os.Stat(encodeDumpFile); os.IsNotExist(err) {
		log.Info(encodeDumpFile, " not exists")
		if _, err = os.Create(encodeDumpFile); err != nil {
			return err
		}
		initFlag = true
	}
	newCluster = &utils.Cluster{
		Nodes: make([]*meta.Node, 0),
	}
	if initFlag {
		for _, node := range fetchData {
			newCluster.Nodes = append(newCluster.Nodes, node)
			atomic.AddUint64(&count, 1)
		}
	} else {
		b, err := ioutil.ReadFile(encodeDumpFile)
		if err != nil {
			return err
		}
		eb, err := encode.Decoding(b)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(eb, newCluster); err != nil {
			return err
		}
		for _, node := range newCluster.Nodes {
			localData[node.Ip] = node
		}
		if len(localData) > 0 {
			for addr, node := range fetchData {
				if _, ok := localData[addr]; !ok {
					newCluster.Nodes = append(newCluster.Nodes, node)
					atomic.AddUint64(&count, 1)
				}
			}
		}

	}
	if count > 0 {
		sort.Slice(newCluster.Nodes, func(i, j int) bool {
			if strings.Compare(newCluster.Nodes[i].Ip, newCluster.Nodes[j].Ip) < 0 {
				return true
			}
			return false
		})
		tempFilePath := fmt.Sprintf("%s.temp", encodeDumpFile)

		if err = newCluster.Dump(tempFilePath); err != nil {
			return err
		}
		if err = os.Rename(tempFilePath, encodeDumpFile); err != nil {
			return err
		}
		log.Info("dump success append nodes:", count)
	}
	return nil
}
func (s *Server) Run() {
	defer s.wg.Done()
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		log.Fatal("failed to listen: %v", err)

	}
	done := make(chan struct{})
	srv := grpc.NewServer()
	pb.RegisterServerNodeServiceServer(srv, s)
	go func(srv *grpc.Server) {
		log.Info("server start at ", s.port)
		if err := srv.Serve(listen); err != nil {
			log.Fatal("failed to server: ", err)
		}
	}(srv)
	go s.reloadAuthorityConfig(done)
	ticker := time.NewTicker(s.timeOut)
	defer ticker.Stop()
	defer srv.Stop()
	for {
		select {
		case <-s.stop:
			done <- struct{}{}
			log.Info("stop server now")
			return
		case <-ticker.C:
			if err = s.internalDump(); err != nil {
				log.Error(err)
			} else {
				log.Info("dump success ")
			}
		}
	}
}

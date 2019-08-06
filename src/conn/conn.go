package conn

import (
	"fmt"
	"pb"
	"strings"
	"utils"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type Conn struct {
	connection *grpc.ClientConn
}

func NewConn(addr string, port int) (*Conn, error) {
	addrInfo := fmt.Sprintf("%s:%d", addr, port)
	conn, err := grpc.Dial(addrInfo, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return &Conn{
		connection: conn,
	}, nil

}
func (a *Conn) Close() {
	a.connection.Close()
}
func (a *Conn) NewUpdateSession(configPath string) (*pb.UpdateResponse, error) {
	c := pb.NewServerNodeServiceClient(a.connection)
	cluster, err := utils.NewCluster(configPath)
	if err != nil {
		return nil, err
	}
	updateRequest, err := cluster.InitRequest()
	if err != nil {
		return nil, err
	}
	uid, err := utils.GetUserName()
	if err != nil {
		return nil, err
	}
	updateRequest.AuthorityUser = strings.ToLower(uid)
	resp, err := c.Load(context.Background(), updateRequest)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
func (a *Conn) NewDumpSession() (*pb.DumpResponse, error) {
	username, err := utils.GetUserName()
	if err != nil {
		return nil, err
	}
	c := pb.NewServerNodeServiceClient(a.connection)

	req := &pb.DumpRequest{
		Username: username,
	}

	return c.Dump(context.Background(), req)
}
func (a *Conn) NewUserSession() (*pb.UserResponse, error) {
	username, err := utils.GetUserName()
	if err != nil {
		return nil, err
	}
	c := pb.NewServerNodeServiceClient(a.connection)

	req := &pb.UserRequest{
		Username: username,
	}

	return c.User(context.Background(), req)
}
func (a *Conn) NewDeleteSession(groups []string) (*pb.DeleteResponse, error) {
	username, err := utils.GetUserName()
	if err != nil {
		return nil, err
	}
	c := pb.NewServerNodeServiceClient(a.connection)

	deleteReq := &pb.DeleteRequest{
		Groups:   groups,
		Username: strings.ToLower(username),
	}

	resp, err := c.Delete(context.Background(), deleteReq)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
func (a *Conn) NewCacheSession() (*pb.CacheResponse, error) {
	username, err := utils.GetUserName()
	if err != nil {
		return nil, err
	}
	c := pb.NewServerNodeServiceClient(a.connection)
	req := &pb.CacheRequest{
		Username: username,
	}
	return c.Cache(context.Background(), req)

}
func (a *Conn) NewViewSession(groupNames []string) (*pb.QueryResponse, error) {
	username, err := utils.GetUserName()
	if err != nil {
		return nil, err
	}
	c := pb.NewServerNodeServiceClient(a.connection)
	queryRequest := &pb.QueryRequest{
		GroupNames: groupNames,
		Username:   strings.ToLower(username),
	}
	resp, err := c.Query(context.Background(), queryRequest)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
func (a *Conn) NewBasicSession() (*pb.BasicResponse, error) {
	username, err := utils.GetUserName()
	if err != nil {
		return nil, err
	}
	c := pb.NewServerNodeServiceClient(a.connection)
	queryRequest := &pb.BasicRequest{
		Username: strings.ToLower(username),
	}
	resp, err := c.Access(context.Background(), queryRequest)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

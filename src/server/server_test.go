package server

import (
	"db"
	"sync"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	defer wg.Wait()
	srv := NewServer(5566, 3, "", wg)
	go srv.Run()
	ticker := time.NewTimer(time.Second * 1)
	for {
		select {
		case <-ticker.C:
			srv.Stop()
			db.DBHandler.Close()
			return
		}
	}
}

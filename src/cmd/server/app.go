package main

import (
	"flag"
	log "logging"
	"os"
	"os/signal"
	"server"
	"sync"
	"syscall"
	"time"
)

const (
	defaultTimeOutMinute = 60
)

var (
	port            = flag.Int("port", 5566, "server running port")
	authorityConfig = flag.String("config", "config.json", "user privileges config")
	dumpMinute      = flag.Int("dump_minute", defaultTimeOutMinute, "time interval for dump cluster")
)

func genTempateConfig(s *server.Server, stop chan struct{}) {
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	defer log.Info("exit genTemplate success")
Loop:
	for {
		select {
		case <-stop:
			break Loop
		case <-ticker.C:
			if _, err := os.Stat(server.DefaultAuthorityConfigFile); os.IsNotExist(err) {
				if err := s.CreateTemplateAuthorityConfig(); err != nil {
					log.Error("Create Template Config:", err)
					break Loop
				}
				log.Info("create template config for privileges success")
			}
		}
	}
}
func main() {
	flag.Parse()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
    done := make(chan struct{})
	wg := &sync.WaitGroup{}

	wg.Add(1)
	defer wg.Wait()

	srv := server.NewServer(*port, *dumpMinute, *authorityConfig, wg)

	go genTempateConfig(srv,done)
	go srv.Run()
	defer log.Info("....vsh_server exit...")
	for {
		select {
		case <-signals:
			srv.Stop()
			done <- struct{}{}
			return
		}
	}

}

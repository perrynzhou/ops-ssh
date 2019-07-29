package main

import (
	"flag"
	log "logging"
	"os"
	"os/signal"
	"server"
	"sync"
	"syscall"
)

const (
	defaultTimeOutMinute = 60
)

var (
	port            = flag.Int("port", 5566, "server running port")
	authorityConfig = flag.String("config", "config.json", "user privileges config")
	dumpMinute      = flag.Int("dump_minute", defaultTimeOutMinute, "time interval for dump cluster")
)

func main() {
	flag.Parse()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	wg := &sync.WaitGroup{}

	wg.Add(1)
	defer wg.Wait()

	srv := server.NewServer(*port, *dumpMinute, *authorityConfig, wg)
	if err := srv.CreateTemplateAuthorityConfig(); err != nil {
		log.Error("CreateTemplateAuthorityConfig:", err)
		return
	}
	go srv.Run()
	defer log.Info("....vsh_server exit...")
	for {
		select {
		case <-signals:
			srv.Stop()
			return
		}
	}

}

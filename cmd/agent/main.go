package main

import (
	"flag"
	"log"
	"os"

	"singbox-node-agent/internal/app"
	"singbox-node-agent/internal/debugmode"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "debug" {
		runDebug()
		return
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func runDebug() {
	fs := flag.NewFlagSet("debug", flag.ExitOnError)

	nodeID := fs.String("node-id", "", "node id, e.g. 213.176.67.109:8812")
	server := fs.String("server", "", "server ip/domain")
	port := fs.Int("port", 0, "server port")
	socksPort := fs.Int("socks-port", 19080, "local socks listen port")
	holdSeconds := fs.Int("hold-seconds", 300, "how long to keep sing-box running")
	configPath := fs.String("config", "", "config path, optional")

	_ = fs.Parse(os.Args[2:])

	opts := debugmode.Options{
		NodeID:      *nodeID,
		Server:      *server,
		Port:        *port,
		SocksPort:   *socksPort,
		HoldSeconds: *holdSeconds,
		ConfigPath:  *configPath,
	}

	if err := debugmode.Run(opts); err != nil {
		log.Fatal(err)
	}
}

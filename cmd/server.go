// CLI to process Kubernetes Specs with a JSON configuration.
//
// Created by M. Massenzio, 2021-02-20

package main

import (
	"flag"
	"fmt"
	log "github.com/massenz/go-statemachine/logging"
	"github.com/massenz/go-statemachine/statemachine"
)

const (
	defaultPort  = 8080
	defaultDebug = false
)

func main() {
	var debug = flag.Bool("debug", defaultDebug,
		"If set, URL handlers will emit a trace log for every request; it may impact performance, "+
			"do not use in production or on heavy load systems")
	var localOnly = flag.Bool("local", false,
		"If set, it only listens to incoming requests from the local host")
	var port = flag.Int("port", defaultPort, "Server port")
	var yamlSpec = flag.String("config", "",
		"State Machine configuration in YAML format")

	flag.Parse()

	logger := log.NewLog()
	if !*debug {
		logger.SetLevel(log.INFO)
	} else {
		logger.SetLevel(log.DEBUG)
		logger.Debug("Emitting DEBUG logs")
	}

	var host = "0.0.0.0"
	if *localOnly {
		logger.Info("Listening on localhost interface only")
		host = "localhost"
	} else {
		logger.Warn("Listening on all interfaces: %s:%d", host, *port)
	}
	if *yamlSpec == "" {
		logger.Fatal(fmt.Errorf("missing YAML configuration file name"))
	}
	logger.Info("Configuring State Machine: %s", *yamlSpec)

	fsm, err := statemachine.NewFSMFromFile(*yamlSpec)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info("Machine configured, in state: %s", fsm.State())
}

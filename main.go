package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/davecgh/go-spew/spew"
	sonos "github.com/ianr0bkny/go-sonos"
	"github.com/ninjasphere/go-ninja"
	"github.com/ninjasphere/go-ninja/logger"
)

const driverName = "driver-sonos"

const (
	DiscoveryPort    = "13104"
	EventingPort     = "13105"
	NetworkInterface = "wlan0"
)

var log = logger.GetLogger(driverName)

func main() {
	log.Infof("Starting %s", driverName)

	conn, err := ninja.Connect("com.ninjablocks.sonos")
	if err != nil {
		log.HandleError(err, "Could not connect to MQTT")
	}

	pwd, _ := os.Getwd()

	_, err = conn.AnnounceDriver("com.ninjablocks.sonos", driverName, pwd)
	if err != nil {
		log.HandleError(err, "Could not get driver bus")
	}

	statusJob, err := ninja.CreateStatusJob(conn, driverName)

	if err != nil {
		log.HandleError(err, "Could not setup status job")
	}

	statusJob.Start()

	mgr, err := sonos.Discover(NetworkInterface, DiscoveryPort)

	if err != nil {
		log.HandleError(err, "Could not configure ssdp manager")
	}

	reactor := sonos.MakeReactor(NetworkInterface, EventingPort)

	if err != nil {
		log.HandleError(err, "Could not configure reactor")
	}

	sonosUnits := sonos.ConnectAny(mgr, reactor, sonos.SVC_CONTENT_DIRECTORY|sonos.SVC_AV_TRANSPORT|sonos.SVC_RENDERING_CONTROL)

	log.Infof(spew.Sprintf("found sonos units %v", sonosUnits))

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	s := <-c
	fmt.Println("Got signal:", s)

}

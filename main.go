package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/ninjasphere/go-ninja/logger"
)

const (
	DiscoveryPort = "13104"
	EventingPort  = "13105"
)

var nlog = logger.GetLogger(info.Name)

func main() {

	log.SetFlags(log.Ltime | log.Lshortfile)

	StartSonosDriver()
	/*nlog.Infof("Starting")

	conn, err := ninja.Connect("com.ninjablocks.sonos")
	if err != nil {
		nlog.HandleError(err, "Could not connect to MQTT")
	}

	pwd, _ := os.Getwd()

	err := conn.ExportDriver()
	if err != nil {
		nlog.HandleError(err, "Could not get driver bus")
	}

	statusJob, err := ninja.CreateStatusJob(conn, driverName)

	if err != nil {
		nlog.HandleError(err, "Could not setup status job")
	}

	statusJob.Start()

	intName, err := GetInterface()

	if nil != err {
		nlog.HandleError(err, "Could not locate interface to bind to")
	}

	nlog.Infof("loading reactor")
	reactor := sonos.MakeReactor(intName, EventingPort)

	zonePlayers, err := detectZP()
	if err != nil {
		nlog.HandleError(err, "Error detecting Sonos ZonePlayers")
	}

	nlog.Infof("Detected %d Sonos ZonePlayers", len(zonePlayers))

	for zone, player := range zonePlayers {
		nlog.Infof(spew.Sprintf("Found %s %v", zone, player))

		unit := sonos.Connect(player, reactor, sonos.SVC_RENDERING_CONTROL|sonos.SVC_AV_TRANSPORT|sonos.SVC_ZONE_GROUP_TOPOLOGY|sonos.SVC_MUSIC_SERVICES)

		_, err := NewPlayer(bus, unit)
		if err != nil {
			nlog.HandleError(err, "failed to register media player")
		}

		nlog.Infof("created media player device %s %s", player.UUID(), player.Name())
	}*/

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	s := <-c
	fmt.Println("Got signal:", s)

}

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/davecgh/go-spew/spew"
	"github.com/ninjasphere/go-ninja"
	"github.com/ninjasphere/go-ninja/logger"
	"github.com/ninjasphere/go-sonos"
	"github.com/ninjasphere/go-sonos/ssdp"
)

const driverName = "driver-sonos"

const (
	DiscoveryPort    = "13104"
	EventingPort     = "13105"
	NetworkInterface = "wlan0"
)

var nlog = logger.GetLogger(driverName)

func detectZP() (zonePlayers ssdp.DeviceMap, err error) {

	nlog.Infof("loading discovery mgr")
	mgr, err := sonos.Discover(NetworkInterface, DiscoveryPort)
	if nil != err {
		return
	}

	zonePlayers = make(ssdp.DeviceMap)
	for uuid, device := range mgr.Devices() {
		if device.Product() == "Sonos" && device.Name() == "ZonePlayer" {
			zonePlayers[uuid] = device
		}
	}
	return
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	nlog.Infof("Starting %s", driverName)

	conn, err := ninja.Connect("com.ninjablocks.sonos")
	if err != nil {
		nlog.HandleError(err, "Could not connect to MQTT")
	}

	pwd, _ := os.Getwd()

	bus, err := conn.AnnounceDriver("com.ninjablocks.sonos", driverName, pwd)
	if err != nil {
		nlog.HandleError(err, "Could not get driver bus")
	}

	statusJob, err := ninja.CreateStatusJob(conn, driverName)

	if err != nil {
		nlog.HandleError(err, "Could not setup status job")
	}

	statusJob.Start()

	nlog.Infof("loading reactor")
	reactor := sonos.MakeReactor(NetworkInterface, EventingPort)

	// debugging the underlying library
	// go func() {
	// 	time.Sleep(30 * time.Second)
	// 	panic("oops")
	// }()

	zonePlayers, err := detectZP()
	if err != nil {
		nlog.HandleError(err, "Error detecting Sonos ZonePlayers")
	}

	nlog.Infof("Detected %d Sonos ZonePlayers", len(zonePlayers))

	for zone, player := range zonePlayers {
		nlog.Infof(spew.Sprintf("Found %s %v", zone, player))

		unit := sonos.Connect(player, reactor, sonos.SVC_RENDERING_CONTROL|sonos.SVC_AV_TRANSPORT|sonos.SVC_ZONE_GROUP_TOPOLOGY|sonos.SVC_MUSIC_SERVICES)

		dev, err := NewPlayer(bus, unit)
		if err != nil {
			nlog.HandleError(err, "failed to register media player")
		}

		nlog.Infof(spew.Sprintf("created media player device %v", dev))
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	s := <-c
	fmt.Println("Got signal:", s)

}

package main

import (
	"fmt"
	"log"
	"net"
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
	DiscoveryPort = "13104"
	EventingPort  = "13105"
)

var nlog = logger.GetLogger(driverName)

func detectZP() (zonePlayers ssdp.DeviceMap, err error) {

	intName, err := GetInterface()

	if nil != err {
		return
	}

	nlog.Infof("loading discovery mgr")
	mgr, err := sonos.Discover(intName, DiscoveryPort)
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

func GetInterface() (intName string, err error) {

	ifaces, err := net.Interfaces()

	if err != nil {
		fmt.Errorf("Failed to get interfaces: %s", err)
		return
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			fmt.Errorf("Failed to get addresses: %s", err)
			return "", err
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if addr.String() != "127.0.0.1/8" && addr.String() != "::1/128" {
					intName = i.Name
				}
			default:
				fmt.Printf("unexpected type %T val %v", v, v)
			}

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

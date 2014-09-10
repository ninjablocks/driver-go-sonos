package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bitly/go-simplejson"
	"github.com/davecgh/go-spew/spew"
	"github.com/ninjasphere/go-ninja"
	"github.com/ninjasphere/go-ninja/devices"
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

func NewPlayer(bus *ninja.DriverBus, sonosUnit *sonos.Sonos) (*devices.MediaPlayerDevice, error) {

	group, _ := sonosUnit.GetZoneGroupAttributes()

	id := group.CurrentZoneGroupID
	name := group.CurrentZoneGroupName

	nlog.Infof("Making media player with ID: %s Label: %s", id, name)

	sigs, _ := simplejson.NewJson([]byte(`{
			"ninja:manufacturer": "Sonos",
			"ninja:productName": "Sonos Player",
			"ninja:productType": "MediaPlayer",
			"ninja:thingType": "MediaPlayer"
	}`))

	deviceBus, err := bus.AnnounceDevice(id, "media-player", name, sigs)
	if err != nil {
		nlog.FatalError(err, "Failed to create media player device bus")
	}

	player, err := devices.CreateMediaPlayerDevice(name, deviceBus)

	if err != nil {
		nlog.FatalError(err, "Failed to create media player device")
	}

	player.ApplyTogglePlay = func() error {
		return sonosUnit.Play(0, "1")
	}

	return player, nil
}

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

	zonePlayers, err := detectZP()
	if err != nil {
		nlog.HandleError(err, "Error detecting Sonos ZonePlayers")
	}

	nlog.Infof("Detected %d Sonos ZonePlayers", len(zonePlayers))

	for zone, player := range zonePlayers {
		nlog.Infof(spew.Sprintf("Found %s %v", zone, player))

		unit := sonos.Connect(player, reactor, sonos.SVC_AV_TRANSPORT|sonos.SVC_ZONE_GROUP_TOPOLOGY|sonos.SVC_MUSIC_SERVICES)

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

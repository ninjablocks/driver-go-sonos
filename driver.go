package main

import (
	"github.com/ninjasphere/go-ninja/api"
	"github.com/ninjasphere/go-ninja/logger"
	"github.com/ninjasphere/go-ninja/model"
	"github.com/ninjasphere/go-sonos"
	"github.com/ninjasphere/go-sonos/ssdp"
	"github.com/ninjasphere/go-sonos/upnp"
)

const (
	DiscoveryPort = 13104
	EventingPort  = 13105
)

var info = ninja.LoadModuleInfo("./package.json")

type sonosDriver struct {
	config    *SonosConfig
	log       *logger.Logger
	players   map[ssdp.UUID]*sonosPlayer
	conn      *ninja.Connection
	reactor   upnp.Reactor
	sendEvent func(event string, payload interface{}) error
}

type SonosConfig struct {
}

func StartSonosDriver() {
	d := &sonosDriver{
		log:     logger.GetLogger(info.Name),
		players: make(map[ssdp.UUID]*sonosPlayer),
	}

	conn, err := ninja.Connect(info.ID)
	if err != nil {
		d.log.Fatalf("Failed to connect to MQTT: %s", err)
	}

	err = conn.ExportDriver(d)

	if err != nil {
		d.log.Fatalf("Failed to export driver: %s", err)
	}

	if nil != err {
		nlog.HandleError(err, "Could not locate interface to bind to")
	}

	d.reactor = sonos.MakeReactor(EventingPort)

	go func() {
		events := d.reactor.Channel()

		d.log.Infof("waiting for events.")

		for {
			_ = <-events

			// because event is a big ball of string it is easier to just iterate over all players
			// and update them all when an event occurs
			for id := range d.players {
				d.log.Infof("Updating %s", id)
				d.players[id].updateState()
			}

			// spew.Dump(event)

			// switch v := event.(type) {
			// case upnp.RenderingControlEvent:
			// 	d.log.Infof(spew.Sprintf("Volume %v", v.LastChange.InstanceID.Volume))
			// case upnp.AVTransportEvent:
			// 	d.log.Infof(spew.Sprintf("TransportState %v", v.LastChange.InstanceID.TransportState))
			// }

			//spew.Dump(event)

		}
	}()

	d.conn = conn
}

func (d *sonosDriver) Start(config *SonosConfig) error {
	d.log.Infof("Starting")
	go d.discover()
	return nil
}

func (d *sonosDriver) Stop() error {
	// TODO: Doesn't aactually stop the devices? Should it?
	return nil
}

func (d *sonosDriver) discover() {

	zonePlayers, err := d.discoverZonePlayers()

	if err != nil {
		d.log.Warningf("Failed to discover ZonePlayers: %s", err)
	} else {
		for uuid, device := range zonePlayers {
			if _, ok := d.players[uuid]; !ok {

				//spew.Dump(device)

				unit := sonos.Connect(device, d.reactor, sonos.SVC_RENDERING_CONTROL|sonos.SVC_AV_TRANSPORT|sonos.SVC_ZONE_GROUP_TOPOLOGY|sonos.SVC_MUSIC_SERVICES)
				//spew.Dump(unit)

				player, err := NewPlayer(d, d.conn, unit)

				if err != nil {

				} else {
					d.players[uuid] = player
				}

			}
		}
	}

}

func (d *sonosDriver) discoverZonePlayers() (zonePlayers ssdp.DeviceMap, err error) {

	d.log.Infof("loading discovery mgr")
	mgr, err := sonos.Discover(DiscoveryPort)

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

func (d *sonosDriver) GetModuleInfo() *model.Module {
	return info
}

func (d *sonosDriver) SetEventHandler(sendEvent func(event string, payload interface{}) error) {
	d.sendEvent = sendEvent
}

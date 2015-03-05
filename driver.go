package main

import (
	"time"

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
	players   map[string]*sonosZonePlayer
	conn      *ninja.Connection
	reactor   upnp.Reactor
	ticker    *time.Ticker
	sendEvent func(event string, payload interface{}) error
}

type SonosConfig struct {
}

func StartSonosDriver() {
	d := &sonosDriver{
		log:     logger.GetLogger(info.Name),
		players: make(map[string]*sonosZonePlayer),
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
			event := <-events

			d.log.Infof("processing event %T", event)

			// because event is a big ball of string it is easier to just iterate over all players
			// and update them all when an event occurs
			for id, player := range d.players {
				d.log.Infof("Updating state for %s", id)
				player.updateState()
			}

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

	if d.ticker != nil {
		d.ticker.Stop()
	}

	d.ticker = time.NewTicker(time.Second * 30)

	go d.discover()
	return nil
}

// S
func (d *sonosDriver) Stop() error {
	// TODO: Doesn't aactually stop the devices? Should it?

	if d.ticker != nil {
		d.ticker.Stop()
	}

	return nil
}

// this function searches for players, retrieves their zone information and builds a map of
// sonos zones keyed on the zones ID.
func (d *sonosDriver) detectZones() (zoneMap map[string]*sonosZoneInfo, err error) {

	zoneMap = make(map[string]*sonosZoneInfo)

	d.log.Infof("loading discovery mgr")
	mgr, err := sonos.Discover(DiscoveryPort)

	if nil != err {
		return
	}

	defer mgr.Close()

	zonePlayers := mgr.Devices()

	// build a list of zones and players
	for _, device := range zonePlayers {

		if !isSonosPlayer(device) {
			continue
		}

		unit := sonos.Connect(device, d.reactor, sonos.SVC_RENDERING_CONTROL|sonos.SVC_AV_TRANSPORT|sonos.SVC_ZONE_GROUP_TOPOLOGY|sonos.SVC_MUSIC_SERVICES)

		// god forbid right..
		if unit == nil {
			continue // skip this one
		}

		groupAttr, err := unit.GetZoneGroupAttributes()

		if err != nil {
			continue // skip this one
		}

		id := groupAttr.CurrentZoneGroupID

		if zoneMap[id] == nil {
			zoneMap[id] = &sonosZoneInfo{
				attributes: groupAttr,
				players:    []*sonos.Sonos{unit},
			}
		} else {
			// append the player
			zoneMap[id].players = append(zoneMap[id].players, unit)
		}

	}

	return
}

type sonosZoneInfo struct {
	attributes *upnp.ZoneGroupAttributes
	players    []*sonos.Sonos
}

func (d *sonosDriver) discover() {

	for t := range d.ticker.C {
		d.log.Debugf("Detecting new players at %s", t)

		zoneMap, err := d.detectZones()

		if err != nil {
			d.log.Warningf("Failed to discover ZonePlayers: %s", err)
			continue
		}

		for uuid, zone := range zoneMap {

			// have we seen this player before?
			if _, ok := d.players[uuid]; !ok {

				player, err := NewPlayer(d, d.conn, zone)

				if err != nil {

				} else {
					d.players[uuid] = player
				}

			} else {
				nlog.Infof("already seen zone player %s", uuid)
				// update the last seen
				d.players[uuid].UpdateLastSeen()
			}

		}

		// detect players that we haven't seen in a while
		for uuid, v := range d.players {
			nlog.Debugf("checking last seen %v", uuid)
			if time.Since(v.lastSeen) > (60 * time.Second) {
				nlog.Warningf("zone player %v is OFFLINE", uuid)
				// TODO publish a notification on a channel that the device is OFFLINE atm
			}
		}
	}

}

func (d *sonosDriver) GetModuleInfo() *model.Module {
	return info
}

func (d *sonosDriver) SetEventHandler(sendEvent func(event string, payload interface{}) error) {
	d.sendEvent = sendEvent
}

func isSonosPlayer(device ssdp.Device) bool {
	return device.Product() == "Sonos" && device.Name() == "ZonePlayer"
}

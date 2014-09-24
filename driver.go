package main

import (
	"fmt"
	"net"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ninjasphere/go-ninja/api"
	"github.com/ninjasphere/go-ninja/logger"
	"github.com/ninjasphere/go-ninja/model"
	"github.com/ninjasphere/go-ninja/rpc"
	"github.com/ninjasphere/go-sonos"
	"github.com/ninjasphere/go-sonos/ssdp"
)

const discoveryInterval = time.Second * 20

var info = ninja.LoadModuleInfo("./package.json")

type sonosDriver struct {
	config    *SonosConfig
	log       *logger.Logger
	players   map[ssdp.UUID]*sonosPlayer
	conn      *ninja.Connection
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

	d.conn = conn
}

func (d *sonosDriver) Start(message *rpc.Message, config *SonosConfig) error {
	d.log.Infof("Starting")
	go d.discover()
	return nil
}

func (d *sonosDriver) Stop(message *rpc.Message) error {
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
				d.log.Infof("Found a new Sonos ZonePlayer: %v", device)

			}
		}
	}

}

func (d *sonosDriver) discoverZonePlayers() (zonePlayers ssdp.DeviceMap, err error) {

	intName, err := GetInterface()

	//intName = "en0"

	if nil != err {
		return nil, err
	}

	d.log.Infof("loading discovery mgr")
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

func (d *sonosDriver) GetModuleInfo() *model.Module {
	return info
}

func (d *sonosDriver) SetEventHandler(sendEvent func(event string, payload interface{}) error) {
	d.sendEvent = sendEvent
}

func GetInterface() (intName string, err error) {

	ifaces, err := net.Interfaces()

	spew.Dump(ifaces, err)

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

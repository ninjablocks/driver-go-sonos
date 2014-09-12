package main

import (
	"github.com/bitly/go-simplejson"
	"github.com/ninjasphere/go-ninja"
	"github.com/ninjasphere/go-ninja/channels"
	"github.com/ninjasphere/go-ninja/devices"
	"github.com/ninjasphere/go-ninja/logger"
	"github.com/ninjasphere/go-sonos"
	"github.com/ninjasphere/go-sonos/upnp"
)

const (
	defaultInstanceID = 0
	defaultSpeed      = "1"
)

type sonosPlayer struct {
	*sonos.Sonos
	log    *logger.Logger
	player *devices.MediaPlayerDevice
}

func (sp *sonosPlayer) applyPlayPause(playing bool) error {

	sp.log.Infof("applyPlayPause called, playing: %t", playing)

	if playing {
		err := sp.Play(defaultInstanceID, defaultSpeed)

		if err != nil {
			return err
		}

		return sp.player.UpdateControlState(channels.MediaControlEventPaused)

	}

	err = sp.Pause(defaultInstanceID)

	if err != nil {
		return err
	}

	return sp.player.UpdateControlState(channels.MediaControlEventPlaying)
}

func (sp *sonosPlayer) applyStop() error {
	sp.log.Infof("applyStop called")

	err := sp.Stop(defaultInstanceID)

	if err != nil {
		return err
	}

	return sp.player.UpdateControlState(channels.MediaControlEventStopped)
}

func (sp *sonosPlayer) applyPlaylistJump(delta int) error {
	sp.log.Infof("applyPlaylistJump called, delta : %d", delta)
	if delta < 0 {
		return sp.Previous(defaultInstanceID)
	}
	return sp.Next(defaultInstanceID)
}

func (sp *sonosPlayer) applyVolume(volume float64) error {
	sp.log.Infof("applyVolume called, volume %f", volume)

	vol := uint16(volume * 100)

	err := sp.SetVolume(defaultInstanceID, upnp.Channel_Master, vol)

	if err != nil {
		return err
	}

	return sp.player.UpdateVolumeState(volume)
}

func (sp *sonosPlayer) applyMuted(muted bool) error {
	err := sp.SetMute(defaultInstanceID, upnp.Channel_Master, muted)

	if err != nil {
		return err
	}

	return sp.player.UpdateMutedState(muted)
}

func (sp *sonosPlayer) bindMethods() {

	sp.player.ApplyPlayPause = sp.applyPlayPause
	sp.player.ApplyStop = sp.applyStop
	sp.player.ApplyPlaylistJump = sp.applyPlaylistJump
	sp.player.ApplyVolume = sp.applyVolume
	sp.player.ApplyMuted = sp.applyMuted

	sp.player.EnableControlChannel([]string{
		"playing",
		"paused",
		"stopped",
		"idle",
	})

	sp.player.EnableVolumeChannel()
}

func (sp *sonosPlayer) updateState() error {

	muted, err := sp.GetMute(defaultInstanceID, upnp.Channel_Master)

	if err != nil {
		return err
	}

	nlog.Infof("UpdateMutedState %t", muted)
	if sp.player.UpdateMutedState(muted); err != nil {
		return err
	}

	vol, err := sp.GetVolume(defaultInstanceID, upnp.Channel_Master)

	if err != nil {
		return err
	}

	var volume float64

	if vol > 0 {
		volume = float64(vol) / 100
	} else {
		volume = float64(0)

	}

	nlog.Infof("UpdateVolumeState %d  %f", vol, volume)
	return sp.player.UpdateVolumeState(volume)
}

func NewPlayer(bus *ninja.DriverBus, sonosUnit *sonos.Sonos) (*sonosPlayer, error) {

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

	sp := &sonosPlayer{sonosUnit, logger.GetLogger("sonosPlayer"), player}

	sp.bindMethods()

	err = sp.updateState()

	if err != nil {
		nlog.FatalError(err, "Failed to create media player device bus")
	}

	return sp, nil
}

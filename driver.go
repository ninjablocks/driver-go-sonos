package main

import (
	"math"

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

var (
	volumeIncrement = uint16(math.Floor(math.MaxUint16 * 0.05))
)

type sonosPlayer struct {
	*sonos.Sonos
	log    *logger.Logger
	player *devices.MediaPlayerDevice
}

func (sp *sonosPlayer) applyTogglePlay() error {

	sp.log.Infof("togglePlay called")

	// get the state of the player now
	info, err := sp.GetTransportInfo(defaultInstanceID)

	if err != nil {
		return err
	}

	if info.CurrentTransportState == "STOPPED" || info.CurrentTransportState == "PAUSED_PLAYBACK" {

		err := sp.Play(defaultInstanceID, defaultSpeed)

		if err != nil {
			return err
		}

		return sp.player.UpdateControlState(channels.MediaControlEventPlaying)
	}

	err = sp.Pause(defaultInstanceID)

	if err != nil {
		return err
	}

	return sp.player.UpdateControlState(channels.MediaControlEventStopped)
}

func (sp *sonosPlayer) applyPlayPause(playing bool) error {

	sp.log.Infof("applyPlayPause called, playing: %s", playing)

	if playing {

		err := sp.Pause(defaultInstanceID)

		if err != nil {
			return err
		}

		return sp.player.UpdateControlState(channels.MediaControlEventPaused)

	}

	err := sp.Play(defaultInstanceID, defaultSpeed)

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
	if delta < defaultInstanceID {
		return sp.Previous(defaultInstanceID)
	}
	return sp.NextSection(defaultInstanceID)
}

func (sp *sonosPlayer) applyVolume(volume float64) error {
	sp.log.Infof("applyVolume called, volume %f", volume)

	vol := uint16(volume * math.MaxUint16)

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

func (sp *sonosPlayer) applyToggleMuted() error {

	// get the state of the player now
	muted, err := sp.GetMute(defaultInstanceID, upnp.Channel_Master)

	if err != nil {
		return err
	}

	if muted {
		err := sp.SetMute(defaultInstanceID, upnp.Channel_Master, false)
		if err != nil {
			return err
		}

		return sp.player.UpdateMutedState(false)
	}

	err = sp.SetMute(defaultInstanceID, upnp.Channel_Master, true)

	if err != nil {
		return err
	}
	return sp.player.UpdateMutedState(true)
}

// applyVolumeDown Decreases the volume by 5%
func (sp *sonosPlayer) applyVolumeDown() error {

	vol, err := sp.GetVolume(defaultInstanceID, upnp.Channel_Master)

	if err != nil {
		return err
	}

	if vol > volumeIncrement {
		vol -= volumeIncrement
	} else {
		vol = 0
	}

	err = sp.SetVolume(defaultInstanceID, upnp.Channel_Master, vol)

	if err != nil {
		return err
	}

	return sp.player.UpdateVolumeState(float64(vol) / float64(math.MaxUint16))
}

// applyVolumeUp Increases the volume by 5%
func (sp *sonosPlayer) applyVolumeUp() error {
	vol, err := sp.GetVolume(defaultInstanceID, upnp.Channel_Master)

	if err != nil {
		return err
	}

	if vol < (math.MaxUint16 - volumeIncrement) {
		vol += volumeIncrement
	} else {
		vol = math.MaxUint16
	}

	err = sp.SetVolume(defaultInstanceID, upnp.Channel_Master, vol)

	if err != nil {
		return err
	}

	return sp.player.UpdateVolumeState(float64(vol) / float64(math.MaxUint16))
}

func (sp *sonosPlayer) bindMethods() {
	sp.player.ApplyTogglePlay = sp.applyTogglePlay
	sp.player.ApplyPlayPause = sp.applyPlayPause
	sp.player.ApplyStop = sp.applyStop
	sp.player.ApplyPlaylistJump = sp.applyPlaylistJump
	sp.player.ApplyVolume = sp.applyVolume
	sp.player.ApplyMuted = sp.applyMuted
	sp.player.ApplyToggleMuted = sp.applyToggleMuted
	sp.player.ApplyVolumeDown = sp.applyVolumeDown
	sp.player.ApplyVolumeUp = sp.applyVolumeUp

	sp.player.EnableControlChannel([]string{
		"playing",
		"paused",
		"stopped",
		"idle",
	})
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

	return sp, nil
}

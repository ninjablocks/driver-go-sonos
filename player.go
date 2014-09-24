package main

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"time"

	"github.com/ninjasphere/go-ninja/api"
	"github.com/ninjasphere/go-ninja/channels"
	"github.com/ninjasphere/go-ninja/devices"
	"github.com/ninjasphere/go-ninja/logger"
	"github.com/ninjasphere/go-ninja/model"
	"github.com/ninjasphere/go-sonos"
	"github.com/ninjasphere/go-sonos/didl"
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

	err := sp.Pause(defaultInstanceID)

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

func (sp *sonosPlayer) applyPlayURL(url string, queue bool) error {
	return fmt.Errorf("Playing a URL has not been implemented yet.")
}

func (sp *sonosPlayer) bindMethods() error {

	sp.player.ApplyPlayPause = sp.applyPlayPause
	sp.player.ApplyStop = sp.applyStop
	sp.player.ApplyPlaylistJump = sp.applyPlaylistJump
	sp.player.ApplyVolume = sp.applyVolume
	sp.player.ApplyMuted = sp.applyMuted
	sp.player.ApplyPlayURL = sp.applyPlayURL

	err := sp.player.EnableControlChannel([]string{
		"playing",
		"paused",
		"stopped",
		"idle",
	})
	if err != nil {
		return err
	}

	err = sp.player.EnableVolumeChannel()
	if err != nil {
		return err
	}

	err = sp.player.EnableMediaChannel()
	if err != nil {
		return err
	}

	return nil
}

var timeDuration = regexp.MustCompile("([0-9]{1,2})\\:([0-9]{2})\\:([0-9]{2})")

func parseDuration(t string) (*time.Duration, error) {

	found := timeDuration.FindAllStringSubmatch(t, -1)

	if found == nil || len(found) == 0 {
		return nil, fmt.Errorf("Failed to parse duration from '%s'", t)
	}

	duration, err := time.ParseDuration(fmt.Sprintf("%sh%sm%ss", found[0][1], found[0][2], found[0][3]))

	if found == nil || len(found) == 0 {
		return nil, fmt.Errorf("Failed to parse duration from '%s': %s", t, err)
	}

	return &duration, nil
}

func (sp *sonosPlayer) updateMedia() error {
	t := sp.log

	positionInfo, err := sp.GetPositionInfo(0)
	if err != nil {
		return err
	}

	if positionInfo.TrackMetaData == "" {
		t.Infof("No track!")
		err = sp.player.UpdateMusicMediaState(nil, nil)
		return err
	}

	duration, err := parseDuration(positionInfo.TrackDuration)
	if err != nil {
		return err
	}

	durationMs := int(*duration / time.Millisecond)

	position, err := parseDuration(positionInfo.RelTime)
	if err != nil {
		return err
	}

	positionMs := int(*position / time.Millisecond)

	var trackMetadata didl.Lite

	err = xml.Unmarshal([]byte(positionInfo.TrackMetaData), &trackMetadata)
	if err != nil {
		return err
	}

	//spew.Dump("DIDL", trackMetadata)

	track := &channels.MusicTrackMediaItem{
		ID:    &positionInfo.TrackURI,
		Title: &trackMetadata.Item[0].Title[0].Value,
		Album: &channels.MediaItemAlbum{
			Name: trackMetadata.Item[0].Album[0].Value,
		},
		Artists: &[]channels.MediaItemArtist{
			channels.MediaItemArtist{
				Name: trackMetadata.Item[0].Creator[0].Value,
			},
		},
		Duration: &durationMs,
	}

	err = sp.player.UpdateMusicMediaState(track, &positionMs)
	if err != nil {
		return err
	}

	return nil
}

func (sp *sonosPlayer) updateState() error {

	/*mediaInfo, err := sp.GetMediaInfo(defaultInstanceID)

	  if err != nil {
	    return err
	  }*/

	//func (d *MediaPlayerDevice) UpdateMusicMediaState(item *MusicTrackMediaItem, position *int) error {
	err := sp.updateMedia()

	if err != nil {
		return fmt.Errorf("Failed to update current media: %s", err)
	}

	muted, err := sp.GetMute(defaultInstanceID, upnp.Channel_Master)

	if err != nil {
		return err
	}

	sp.log.Infof("UpdateMutedState %t", muted)
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

	sp.log.Infof("UpdateVolumeState %d  %f", vol, volume)
	return sp.player.UpdateVolumeState(volume)
}

func NewPlayer(driver *sonosDriver, conn *ninja.Connection, sonosUnit *sonos.Sonos) (*sonosPlayer, error) {

	group, _ := sonosUnit.GetZoneGroupAttributes()

	id := group.CurrentZoneGroupID
	name := group.CurrentZoneGroupName

	nlog.Infof("Making media player with ID: %s Label: %s", id, name)

	player, err := devices.CreateMediaPlayerDevice(driver, &model.Device{
		ID:     id,
		IDType: "sonos",
		Name:   &name,
		Signatures: &map[string]string{
			"ninja:manufacturer": "Sonos",
			"ninja:productName":  "Sonos Player",
			"ninja:productType":  "MediaPlayer",
			"ninja:thingType":    "MediaPlayer",
		},
	}, conn)

	if err != nil {
		nlog.FatalError(err, "Failed to create media player device")
	}

	sp := &sonosPlayer{sonosUnit, logger.GetLogger("sonosPlayer"), player}

	err = sp.bindMethods()
	if err != nil {
		sp.log.FatalError(err, "Failed to bind channels to sonos device")
	}

	err = sp.updateState()

	if err != nil {
		sp.log.FatalError(err, "Failed to create media player device bus")
	}

	return sp, nil
}

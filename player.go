package main

import (
	"encoding/xml"
	"fmt"
	"math"
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

type sonosZonePlayer struct {
	*sonos.Sonos
	log      *logger.Logger
	player   *devices.MediaPlayerDevice
	lastSeen time.Time
}

func (sp *sonosZonePlayer) UpdateLastSeen() {
	sp.lastSeen = time.Now()
}

func (sp *sonosZonePlayer) applyPlayPause(playing bool) error {

	sp.log.Infof("applyPlayPause called, playing: %t", playing)

	if playing {
		err := sp.Play(defaultInstanceID, defaultSpeed)

		if err != nil {
			return err
		}

		return sp.player.UpdateControlState(channels.MediaControlEventPlaying)

	}

	err := sp.Pause(defaultInstanceID)

	if err != nil {
		return err
	}

	return sp.player.UpdateControlState(channels.MediaControlEventPaused)
}

func (sp *sonosZonePlayer) applyStop() error {
	sp.log.Infof("applyStop called")

	err := sp.Stop(defaultInstanceID)

	if err != nil {
		return err
	}

	return sp.player.UpdateControlState(channels.MediaControlEventStopped)
}

func (sp *sonosZonePlayer) applyPlaylistJump(delta int) error {
	sp.log.Infof("applyPlaylistJump called, delta : %d", delta)
	if delta < 0 {
		return sp.Previous(defaultInstanceID)
	}
	return sp.Next(defaultInstanceID)
}

func (sp *sonosZonePlayer) applyVolume(volume *channels.VolumeState) error {

	sp.log.Infof("applyVolume called, volume %v", volume)

	if volume.Level != nil {
		vol := uint16(*volume.Level * 100)

		if err := sp.SetVolume(defaultInstanceID, upnp.Channel_Master, vol); err != nil {
			return err
		}
	}

	if volume.Muted != nil {

		if err := sp.SetMute(defaultInstanceID, upnp.Channel_Master, *volume.Muted); err != nil {
			return err
		}
	}

	return sp.player.UpdateVolumeState(volume)
}

func (sp *sonosZonePlayer) applyPlayURL(url string, queue bool) error {
	return fmt.Errorf("Playing a URL has not been implemented yet.")
}

func (sp *sonosZonePlayer) bindMethods() error {

	sp.player.ApplyPlayPause = sp.applyPlayPause
	sp.player.ApplyStop = sp.applyStop
	sp.player.ApplyPlaylistJump = sp.applyPlaylistJump
	sp.player.ApplyVolume = sp.applyVolume
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

	err = sp.player.EnableVolumeChannel(true)
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

func (sp *sonosZonePlayer) updateMedia() error {
	t := sp.log

	positionInfo, err := sp.GetPositionInfo(0)
	if err != nil {
		return fmt.Errorf("Failed getting position info: %s", err)
	}

	if positionInfo.TrackMetaData == "" {
		t.Infof("No track!")
		err = sp.player.UpdateMusicMediaState(nil, nil)
		return err
	}

	track := &channels.MusicTrackMediaItem{
		ID: &positionInfo.TrackURI,
	}

	var positionMs int

	if positionInfo.TrackDuration != "NOT_IMPLEMENTED" {

		duration, err := parseDuration(positionInfo.TrackDuration)
		if err != nil {
			return fmt.Errorf("Failed parsing duration: %s", err)
		}

		durationMs := int(*duration / time.Millisecond)
		track.Duration = &durationMs
	}

	if positionInfo.RelTime != "NOT_IMPLEMENTED" {
		position, err := parseDuration(positionInfo.RelTime)
		if err != nil {
			return fmt.Errorf("Failed parsing position: %s", err)
		}

		positionMs = int(*position / time.Millisecond)
	}

	var trackMetadata didl.Lite

	if positionInfo.TrackMetaData != "NOT_IMPLEMENTED" {

		err = xml.Unmarshal([]byte(positionInfo.TrackMetaData), &trackMetadata)
		if err != nil {
			return fmt.Errorf("Failed unmarshalling metadata(%s): %s", positionInfo.TrackMetaData, err)
		}
		if len(trackMetadata.Item) > 0 {
			item := trackMetadata.Item[0]

			if len(item.Title) > 0 {
				track.Title = &item.Title[0].Value
			}

			if len(item.Album) > 0 {
				track.Album = &channels.MediaItemAlbum{
					Name: item.Album[0].Value,
				}
			}

			if len(item.Creator) > 0 {
				track.Artists = &[]channels.MediaItemArtist{
					channels.MediaItemArtist{
						Name: item.Creator[0].Value,
					},
				}
			}

		}
	}

	err = sp.player.UpdateMusicMediaState(track, &positionMs)
	if err != nil {
		return fmt.Errorf("Failed sending media state: %s", err)
	}

	return nil
}

func (sp *sonosZonePlayer) updateState() error {

	sp.log.Infof("updateMedia")
	if err := sp.updateMedia(); err != nil {
		return fmt.Errorf("Failed to update media state: %s", err)
	}

	muted, err := sp.GetMute(defaultInstanceID, upnp.Channel_Master)

	if err != nil {
		return fmt.Errorf("Failed to get mute: %s", err)
	}

	vol, err := sp.GetVolume(defaultInstanceID, upnp.Channel_Master)

	if err != nil {
		return fmt.Errorf("Failed to get volume: %s", err)
	}

	var volume float64

	if vol > 0 {
		volume = float64(vol) / 100
	} else {
		volume = float64(0)
	}

	volume = math.Min(1, volume)

	sp.log.Infof("UpdateVolumeState %d  %f", vol, volume)

	if err := sp.player.UpdateVolumeState(&channels.VolumeState{
		Level: &volume,
		Muted: &muted,
	}); err != nil {
		return err
	}

	transportInfo, err := sp.GetTransportInfo(defaultInstanceID)

	if err != nil {
		return err
	}

	switch transportInfo.CurrentTransportState {
	case upnp.State_PLAYING:
		sp.log.Infof("UpdateControlState PLAYING")
		return sp.player.UpdateControlState(channels.MediaControlEventPlaying)
	case upnp.State_STOPPED:
		sp.log.Infof("UpdateControlState STOPPED")
		return sp.player.UpdateControlState(channels.MediaControlEventStopped)
	case upnp.State_PAUSED_PLAYBACK:
		sp.log.Infof("UpdateControlState PAUSED")
		return sp.player.UpdateControlState(channels.MediaControlEventPaused)
	}

	return nil
}

func NewPlayer(driver *sonosDriver, conn *ninja.Connection, zoneInfo *sonosZoneInfo) (*sonosZonePlayer, error) {

	id := zoneInfo.attributes.CurrentZoneGroupID
	name := zoneInfo.attributes.CurrentZoneGroupName

	nlog.Infof("Making media player with ID: %s Label: %s")

	player, err := devices.CreateMediaPlayerDevice(driver, &model.Device{
		NaturalID:     id,
		NaturalIDType: "sonos",
		Name:          &name,
		Signatures: &map[string]string{
			"ninja:manufacturer": "Sonos",
			"ninja:productName":  "Sonos Player",
			"ninja:productType":  "MediaPlayer",
			"ninja:thingType":    "mediaplayer",
		},
	}, conn)

	if err != nil {
		nlog.FatalError(err, "Failed to create media player device")
	}

	// At the moment I am using the first player in that zone
	// i need to know how this looks in a multi player situation.
	sonosUnit := zoneInfo.players[0]

	sp := &sonosZonePlayer{sonosUnit, logger.GetLogger("sonosZonePlayer"), player, time.Now()}

	err = sp.bindMethods()
	if err != nil {
		sp.log.FatalError(err, "Failed to bind channels to sonos device")
	}

	err = sp.updateState()

	if err != nil {
		sp.log.FatalError(err, "Failed to get initial device state")
	}

	return sp, nil
}

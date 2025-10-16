package model

import (
	"context"
	"fmt"
)

// TODO: Should the type be encoded in the ID?
func GetEntityByID(ctx context.Context, ds DataStore, id string) (interface{}, error) {
	if discID, trackID, ok := ParseDiscoveryStreamID(id); ok {
		entry, err := ds.Discovery(ctx).Tracks(discID).Read(trackID)
		if err != nil {
			return nil, err
		}
		track, ok := entry.(DiscoveryTrack)
		if !ok {
			return nil, fmt.Errorf("unexpected discovery track type %T", entry)
		}
		return track.ToMediaFile()
	}

	ar, err := ds.Artist(ctx).Get(id)
	if err == nil {
		return ar, nil
	}
	al, err := ds.Album(ctx).Get(id)
	if err == nil {
		return al, nil
	}
	disc, err := ds.Discovery(ctx).Get(id)
	if err == nil {
		return disc, nil
	}
	pls, err := ds.Playlist(ctx).Get(id)
	if err == nil {
		return pls, nil
	}
	mf, err := ds.MediaFile(ctx).Get(id)
	if err == nil {
		return mf, nil
	}
	return nil, err
}

package nativeapi

import "testing"

func TestResolveRetailPlayerChannelName(t *testing.T) {
	tests := []struct {
		name      string
		channelID string
		catalog   map[string]retailPlayerChannelCatalogEntry
		want      string
	}{
		{
			name:      "empty catalog",
			channelID: "channel-1",
			catalog:   map[string]retailPlayerChannelCatalogEntry{},
			want:      "",
		},
		{
			name:      "single catalog entry without channel id",
			channelID: "",
			catalog: map[string]retailPlayerChannelCatalogEntry{
				"abc": {ID: "abc", Name: "Channel A"},
			},
			want: "Channel A",
		},
		{
			name:      "multiple channels match by key",
			channelID: "channel-2",
			catalog: map[string]retailPlayerChannelCatalogEntry{
				"channel-1": {ID: "channel-1", Name: "Channel One"},
				"channel-2": {ID: "channel-2", Name: "Channel Two"},
			},
			want: "Channel Two",
		},
		{
			name:      "multiple channels no match",
			channelID: "channel-3",
			catalog: map[string]retailPlayerChannelCatalogEntry{
				"channel-1": {ID: "channel-1", Name: "Channel One"},
				"channel-2": {ID: "channel-2", Name: "Channel Two"},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveRetailPlayerChannelName(tt.channelID, tt.catalog); got != tt.want {
				t.Fatalf("resolveRetailPlayerChannelName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyRetailPlayerChannelNameFallbackFromMap(t *testing.T) {
	devices := []retailPlayerDevice{
		{ID: "device-1", ChannelName: ""},
		{ID: "device-2", ChannelName: "Catalog Name"},
		{ID: "device-3", ChannelName: ""},
		{ID: "", ChannelName: ""},
	}

	channelNames := map[string]string{
		"device-1": "Fallback One",
		"device-3": "",
	}

	applyRetailPlayerChannelNameFallbackFromMap(devices, channelNames)

	if devices[0].ChannelName != "Fallback One" {
		t.Fatalf("expected device-1 channel name fallback, got %q", devices[0].ChannelName)
	}
	if devices[1].ChannelName != "Catalog Name" {
		t.Fatalf("expected device-2 catalog channel name to remain, got %q", devices[1].ChannelName)
	}
	if devices[2].ChannelName != "" {
		t.Fatalf("expected device-3 channel name to remain empty, got %q", devices[2].ChannelName)
	}
	if devices[3].ChannelName != "" {
		t.Fatalf("expected device with empty id to remain empty, got %q", devices[3].ChannelName)
	}
}

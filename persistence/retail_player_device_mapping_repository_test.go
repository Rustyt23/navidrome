package persistence

import "testing"

func TestUniqueRetailPlayerDeviceSlug(t *testing.T) {
	t.Run("keeps unclaimed slug", func(t *testing.T) {
		owners := map[string]string{}

		slug := uniqueRetailPlayerDeviceSlug("lobby", "38f26227-e25c-4c79-a1fd-0269dca43d56", owners)

		if slug != "lobby" {
			t.Fatalf("expected slug lobby, got %q", slug)
		}
	})

	t.Run("keeps slug already owned by same device", func(t *testing.T) {
		owners := map[string]string{
			"lobby": "38f26227-e25c-4c79-a1fd-0269dca43d56",
		}

		slug := uniqueRetailPlayerDeviceSlug("lobby", "38f26227-e25c-4c79-a1fd-0269dca43d56", owners)

		if slug != "lobby" {
			t.Fatalf("expected slug lobby, got %q", slug)
		}
	})

	t.Run("adds stable suffix when another device owns slug", func(t *testing.T) {
		owners := map[string]string{
			"lobby": "existing-device",
		}

		slug := uniqueRetailPlayerDeviceSlug("lobby", "38f26227-e25c-4c79-a1fd-0269dca43d56", owners)

		if slug != "lobby-38f26227" {
			t.Fatalf("expected suffixed slug, got %q", slug)
		}
	})
}

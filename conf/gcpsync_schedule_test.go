package conf_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/scheduler"
	"github.com/spf13/viper"
)

func prepareGCPSyncScheduleConfig(t *testing.T) {
	t.Helper()
	savedViper := *viper.GetViper()
	savedServer := conf.Server
	t.Cleanup(func() {
		conf.Server = savedServer
		*viper.GetViper() = savedViper
	})
	viper.Reset()
	conf.ResetConf()
	conf.SetViperDefaults()
	viper.SetDefault("datafolder", t.TempDir())
	viper.SetDefault("loglevel", "error")
	t.Cleanup(conf.SetLogFatal(func(args ...any) {
		panic(fmt.Sprint(args...))
	}))
}

func TestGCPSyncScheduleDefaultsToManual(t *testing.T) {
	prepareGCPSyncScheduleConfig(t)
	conf.Load(true)
	if got := conf.Server.GCPSync.Schedule; got != "0" {
		t.Fatalf("schedule default = %q, want manual mode 0", got)
	}
}

func TestGCPSyncScheduleNormalizesAndAcceptsActiveModes(t *testing.T) {
	for _, schedule := range []string{"", " \t ", "0", " 0 ", "@every 10m", "10m", " CRON_TZ=Asia/Kolkata 0 2 * * * "} {
		t.Run(schedule, func(t *testing.T) {
			prepareGCPSyncScheduleConfig(t)
			viper.Set("gcpsync.enabled", true)
			viper.Set("gcpsync.autoupload", true)
			viper.Set("gcpsync.schedule", schedule)
			conf.Load(true)
			if got := conf.Server.GCPSync.Schedule; got != strings.TrimSpace(schedule) {
				t.Fatalf("schedule = %q, want trimmed value %q", got, strings.TrimSpace(schedule))
			}
		})
	}
}

func TestGCPSyncScheduleTimezoneControlsNextUploadTime(t *testing.T) {
	prepareGCPSyncScheduleConfig(t)
	viper.Set("gcpsync.enabled", true)
	viper.Set("gcpsync.autoupload", true)
	viper.Set("gcpsync.schedule", "CRON_TZ=Asia/Kolkata 0 2 * * *")
	conf.Load(true)
	schedule, err := scheduler.ParseCrontab(conf.Server.GCPSync.Schedule)
	if err != nil {
		t.Fatal(err)
	}
	// 18:00 UTC is 23:30 in Kolkata; the next 02:00 there is 20:30 UTC.
	from := time.Date(2026, 10, 10, 18, 0, 0, 0, time.UTC)
	want := time.Date(2026, 10, 10, 20, 30, 0, 0, time.UTC)
	if next := schedule.Next(from); !next.Equal(want) {
		t.Fatalf("next upload = %s, want %s", next, want)
	}
}

func TestGCPSyncScheduleRejectsInvalidActiveCron(t *testing.T) {
	for _, schedule := range []string{"61 2 * * *", "CRON_TZ=Not/AZone 0 2 * * *"} {
		t.Run(schedule, func(t *testing.T) {
			prepareGCPSyncScheduleConfig(t)
			viper.Set("gcpsync.enabled", true)
			viper.Set("gcpsync.autoupload", true)
			viper.Set("gcpsync.schedule", schedule)
			defer func() {
				failure := recover()
				if failure == nil || !strings.Contains(fmt.Sprint(failure), "invalid GCPSync.Schedule") {
					t.Fatalf("configuration failure = %v, want invalid GCPSync.Schedule", failure)
				}
			}()
			conf.Load(true)
		})
	}
}

func TestGCPSyncScheduleDoesNotValidateInactiveCron(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled_%t", enabled), func(t *testing.T) {
			prepareGCPSyncScheduleConfig(t)
			viper.Set("gcpsync.enabled", enabled)
			viper.Set("gcpsync.autoupload", !enabled)
			viper.Set("gcpsync.schedule", "not a cron")
			conf.Load(true)
			if got := conf.Server.GCPSync.Schedule; got != "not a cron" {
				t.Fatalf("inactive schedule = %q, want preserved value", got)
			}
		})
	}
}

func TestGCPSyncScheduleTOMLOverridesSourceDefault(t *testing.T) {
	for _, fileSchedule := range []string{"0", "CRON_TZ=Asia/Kolkata 0 2 * * *"} {
		t.Run(fileSchedule, func(t *testing.T) {
			prepareGCPSyncScheduleConfig(t)
			sourceDefault := "0"
			if fileSchedule == "0" {
				sourceDefault = "CRON_TZ=Asia/Kolkata 0 2 * * *"
			}
			viper.SetDefault("gcpsync.schedule", sourceDefault)
			path := filepath.Join(t.TempDir(), "schedule.toml")
			content := fmt.Sprintf("[GCPSync]\nEnabled = true\nAutoUpload = true\nSchedule = %q\n", fileSchedule)
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			conf.InitConfig(path, false)
			conf.Load(true)
			if got := conf.Server.GCPSync.Schedule; got != fileSchedule {
				t.Fatalf("schedule = %q, want TOML %q over source default %q", got, fileSchedule, sourceDefault)
			}
		})
	}
}

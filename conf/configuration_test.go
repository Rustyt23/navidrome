package conf_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/navidrome/navidrome/conf"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

func TestConfiguration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Configuration Suite")
}

var _ = Describe("Configuration", func() {
	BeforeEach(func() {
		// Reset viper configuration
		viper.Reset()
		conf.SetViperDefaults()
		viper.SetDefault("datafolder", GinkgoT().TempDir())
		viper.SetDefault("loglevel", "error")
		conf.ResetConf()
	})

	It("sets the loudness normalization tolerance default", func() {
		conf.Load(true)

		Expect(conf.Server.Scanner.LoudnessNormalization.Tolerance).To(Equal(conf.DefaultLoudnessNormalizationTolerance))
	})

	It("loads a 0.1 loudness normalization tolerance from configuration", func() {
		filename := filepath.Join(GinkgoT().TempDir(), "navidrome.toml")
		content := "[Scanner.LoudnessNormalization]\nTolerance = 0.1\n"
		Expect(os.WriteFile(filename, []byte(content), 0600)).To(Succeed())

		conf.InitConfig(filename)
		conf.Load(true)

		Expect(conf.Server.Scanner.LoudnessNormalization.Tolerance).To(Equal(0.1))
	})

	DescribeTable("should load configuration from",
		func(format string) {
			filename := filepath.Join("testdata", "cfg."+format)

			// Initialize config with the test file
			conf.InitConfig(filename)
			// Load the configuration (with noConfigDump=true)
			conf.Load(true)

			// Execute the format-specific assertions
			Expect(conf.Server.MusicFolder).To(Equal(fmt.Sprintf("/%s/music", format)))
			Expect(conf.Server.UIWelcomeMessage).To(Equal("Welcome " + format))
			Expect(conf.Server.Tags["custom"].Aliases).To(Equal([]string{format, "test"}))

			// The config file used should be the one we created
			Expect(conf.Server.ConfigFile).To(Equal(filename))
		},
		Entry("TOML format", "toml"),
		Entry("YAML format", "yaml"),
		Entry("INI format", "ini"),
		Entry("JSON format", "json"),
	)
})

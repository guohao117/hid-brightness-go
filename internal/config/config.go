package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3"
)

type Config struct {
	MinLux          float64
	MaxLux          float64
	Alpha           float64
	Gamma           float64
	MinBrightness   float64
	SlewRate        float64
	TargetThreshold float64
}

var DefaultConfig = Config{
	MinLux:          0.0,
	MaxLux:          3000.0,
	Alpha:           1.0,
	Gamma:           2.2,
	MinBrightness:   5.0,
	SlewRate:        4.0,
	TargetThreshold: 1.0,
}

func LoadConfig(args []string) (*Config, error) {
	fs := flag.NewFlagSet("hid-brightness", flag.ExitOnError)

	conf := DefaultConfig
	fs.Float64Var(&conf.MinLux, "min-lux", DefaultConfig.MinLux, "Minimum Lux value")
	fs.Float64Var(&conf.MaxLux, "max-lux", DefaultConfig.MaxLux, "Maximum Lux value")
	fs.Float64Var(&conf.Alpha, "alpha", DefaultConfig.Alpha, "Alpha curve parameter")
	fs.Float64Var(&conf.Gamma, "gamma", DefaultConfig.Gamma, "Gamma correction parameter")
	fs.Float64Var(&conf.MinBrightness, "min-brightness", DefaultConfig.MinBrightness, "Minimum brightness percentage")
	fs.Float64Var(&conf.SlewRate, "slew-rate", DefaultConfig.SlewRate, "Brightness transition speed")
	fs.Float64Var(&conf.TargetThreshold, "target-threshold", DefaultConfig.TargetThreshold, "Lux change threshold for update")

	var configPath string
	fs.StringVar(&configPath, "config", "config.txt", "path to config file (plain text)")

	err := ff.Parse(fs, args,
		ff.WithEnvVarPrefix("HID_BRIGHTNESS"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithAllowMissingConfigFile(true),
	)

	return &conf, err
}

func SaveConfig(path string, conf *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "min-lux %.1f\n", conf.MinLux)
	fmt.Fprintf(f, "max-lux %.1f\n", conf.MaxLux)
	fmt.Fprintf(f, "alpha %.1f\n", conf.Alpha)
	fmt.Fprintf(f, "gamma %.1f\n", conf.Gamma)
	fmt.Fprintf(f, "min-brightness %.1f\n", conf.MinBrightness)
	fmt.Fprintf(f, "slew-rate %.1f\n", conf.SlewRate)
	fmt.Fprintf(f, "target-threshold %.1f\n", conf.TargetThreshold)

	return nil
}

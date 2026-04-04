package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"regexp"

	"github.com/adhocore/gronx"
	shoutrrrRouter "github.com/containrrr/shoutrrr/pkg/router"
	shoutrrrTypes "github.com/containrrr/shoutrrr/pkg/types"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

type RawConfig struct {
	Verbose      bool
	Cron         string
	HealthCheck  RawHealthCheck `mapstructure:"health_check"`
	Notifiers    []RawNotifier  `mapstructure:"notifiers"`
	IgnoredLinks []string       `mapstructure:"ignored_links"`
	Sites        []RawSiteConfig
}

type RawHealthCheck struct {
	URL string
}

type RawNotifier struct {
	Name         string
	URL          string
	TemplateName string `mapstructure:"template_name"`
}

type RawSiteConfig struct {
	URL          string
	IgnoredLinks []string `mapstructure:"ignored_links"`
	Notify       []string
}

type Config struct {
	RawConfig
	HealthCheck  HealthCheck
	Notifiers    map[string]NotifierConfig
	IgnoredLinks []*regexp.Regexp
	Sites        []SiteConfig
}

type HealthCheck struct {
	URL *url.URL
}

type NotifierConfig struct {
	Notifier     shoutrrrTypes.Sender
	NotifierName string
	TemplateName string
}

type SiteConfig struct {
	URL          *url.URL
	IgnoredLinks []*regexp.Regexp
	Notify       []string
}

// ParseConfig tries to parse the provided config path into a DeLiC config object.
func ParseConfig(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	err := viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("failed reading config file: %w", err)
	}

	// Bind ENV variables
	err = bindEnvs([]envBinding{{"Verbose", "VERBOSE"}})
	if err != nil {
		return nil, err
	}

	// Unmarshal to Config struct
	var rawConfig RawConfig
	err = viper.Unmarshal(&rawConfig, viper.DecodeHook(mapstructure.StringToSliceHookFunc(",")))
	if err != nil {
		return nil, fmt.Errorf("unable to decode config into struct: %w", err)
	}

	// Validate and convert config
	config := Config{RawConfig: rawConfig}
	if len(rawConfig.Sites) == 0 {
		return nil, ErrNoSitesDefined
	}
	if rawConfig.Cron != "" {
		gron := gronx.New()
		if !gron.IsValid(rawConfig.Cron) {
			return nil, InvalidCronSpecError{rawConfig.Cron}
		}
	}
	if rawConfig.HealthCheck.URL != "" {
		config.HealthCheck.URL, err = url.Parse(rawConfig.HealthCheck.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse health check URL '%s': %w", rawConfig.HealthCheck.URL, err)
		}
	} else {
		slog.Info("No health check URL found, disabling health check calling.")
	}
	config.Notifiers = make(map[string]NotifierConfig, len(rawConfig.Notifiers))
	serviceRouter := &shoutrrrRouter.ServiceRouter{}
	for i, rawNotifier := range rawConfig.Notifiers {
		if rawNotifier.Name == "" {
			return nil, NotifierFieldMissingError{field: "name", index: i}
		}
		if rawNotifier.URL == "" {
			return nil, NotifierFieldMissingError{field: "url", index: i}
		}
		notifierService, err := serviceRouter.Locate(rawNotifier.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse url of notifier at index %d: %s: %w", i, rawNotifier.URL, err)
		}
		config.Notifiers[rawNotifier.Name] = NotifierConfig{
			Notifier:     notifierService,
			NotifierName: rawNotifier.Name,
			TemplateName: rawNotifier.TemplateName,
		}
	}

	// Parse global ignored links
	config.IgnoredLinks, err = parseIgnoredLinks(rawConfig.IgnoredLinks, "global")
	if err != nil {
		return nil, err
	}

	config.Sites = make([]SiteConfig, 0, len(rawConfig.Sites))
	for _, rawSite := range rawConfig.Sites {
		// Create initial SiteConfig
		site := SiteConfig{
			URL:          nil,
			IgnoredLinks: make([]*regexp.Regexp, 0, len(rawSite.IgnoredLinks)),
			Notify:       rawSite.Notify,
		}

		// Parse URL
		site.URL, err = url.Parse(rawSite.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse site '%s': %w", site, err)
		}

		// Parse IgnoredLinks
		site.IgnoredLinks, err = parseIgnoredLinks(rawSite.IgnoredLinks, site.URL.String())
		if err != nil {
			return nil, err
		}

		// Validate Notify
		for _, notify := range rawSite.Notify {
			if _, ok := config.Notifiers[notify]; !ok {
				return nil, UnknownNotifierForSiteError{siteURL: site.URL.String(), notifier: notify}
			}
		}

		// Validation successful
		config.Sites = append(config.Sites, site)
	}
	return &config, nil
}

type envBinding struct {
	configPath string
	envName    string
}

func bindEnvs(bindings []envBinding) error {
	for _, binding := range bindings {
		err := viper.BindEnv(binding.configPath, binding.envName)
		if err != nil {
			return fmt.Errorf("failed to bind env var %s to %s: %w", binding.envName, binding.configPath, err)
		}
	}
	return nil
}

func parseIgnoredLinks(links []string, site string) ([]*regexp.Regexp, error) {
	parsedLinks := make([]*regexp.Regexp, len(links))
	for i, link := range links {
		linkRegex, err := regexp.Compile(link)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ignored link '%s' for site '%s': %w", link, site, err)
		}
		parsedLinks[i] = linkRegex
	}
	return parsedLinks, nil
}

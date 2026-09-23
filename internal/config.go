package internal

import (
	"os"
	"path"

	"github.com/lithammer/fuzzysearch/fuzzy"
	. "github.com/theurichde/go-aws-sso/pkg/sso"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// SsoInstance describes a single SSO organization: its start-url and the AWS region hosting
// its SSO/Identity Center portal. Name is optional and only used for display/log purposes.
type SsoInstance struct {
	Name     string `yaml:"name,omitempty"`
	StartUrl string `yaml:"start-url"`
	Region   string `yaml:"region"`
}

type AppConfig struct {
	StartUrl     string        `yaml:"start-url,omitempty"`
	Region       string        `yaml:"region,omitempty"`
	SsoInstances []SsoInstance `yaml:"sso-instances,omitempty"`
}

// Instances returns the configured SSO instances. If the new SsoInstances list is populated it
// is returned as-is; otherwise, for backward compatibility with a legacy single start-url/region
// config file, a one-element list is synthesized from the legacy fields. Returns an empty slice
// if nothing is configured at all.
func (ac AppConfig) Instances() []SsoInstance {
	if len(ac.SsoInstances) > 0 {
		return ac.SsoInstances
	}
	if ac.StartUrl != "" {
		return []SsoInstance{{StartUrl: ac.StartUrl, Region: ac.Region}}
	}
	return nil
}

func promptStartUrl(prompt Prompt, dfault string) string {
	return prompt.Prompt("SSO Start URL", dfault)
}

func promptRegion(prompt Prompt) string {
	_, region := prompt.Select("Select your AWS Region. Hint: FuzzySearch supported", AwsRegions, func(input string, index int) bool {
		target := AwsRegions[index]
		return fuzzy.MatchFold(input, target)
	})
	return region
}

// GenerateConfigAction generates a fresh config file, overwriting any existing one.
// If --start-url/--region flags are given, a single SSO instance is written non-interactively
// (preserving today's scripted `config generate -u -r` usage). Otherwise the user is prompted
// interactively and may add, import or remove SSO start-urls via runInstanceEditor.
func GenerateConfigAction(context *cli.Context) error {

	prompter := Prompter{}
	startUrl := context.String("start-url")
	region := context.String("region")

	var instances []SsoInstance

	if startUrl != "" || region != "" {
		if startUrl == "" {
			startUrl = promptStartUrl(prompter, "")
		}
		if region == "" {
			region = promptRegion(prompter)
		}
		instances = []SsoInstance{{StartUrl: startUrl, Region: region}}
	} else {
		instances = runInstanceEditor(prompter, nil)
	}

	appConfig := AppConfig{SsoInstances: instances}

	configFile := ConfigFilePath()
	err := writeConfig(configFile, appConfig)
	return err
}

// EditConfigAction lets the user add, import or remove SSO instances from the existing config file.
func EditConfigAction(_ *cli.Context) error {

	prompter := Prompter{}
	config := ReadConfig(ConfigFilePath())
	instances := runInstanceEditor(prompter, config.Instances())

	newConfig := AppConfig{SsoInstances: instances}
	err := writeConfig(ConfigFilePath(), newConfig)
	check(err)
	return err

}

// runInstanceEditor drives an interactive menu for building up a list of SSO instances: adding
// one manually, importing all discovered ones from ~/.aws/config, removing one, or finishing.
// Shared by GenerateConfigAction (when no -u/-r flags are given) and EditConfigAction.
func runInstanceEditor(prompter Prompt, instances []SsoInstance) []SsoInstance {
editLoop:
	for {
		options := []string{"Add a new SSO start URL", "Import SSO start-urls from ~/.aws/config"}
		if len(instances) > 0 {
			options = append(options, "Remove an SSO start URL")
		}
		options = append(options, "Done")

		_, choice := prompter.Select("Configure SSO start URLs", options, func(input string, index int) bool {
			return fuzzy.MatchFold(input, options[index])
		})

		switch choice {
		case "Add a new SSO start URL":
			instances = append(instances, SsoInstance{
				StartUrl: promptStartUrl(prompter, ""),
				Region:   promptRegion(prompter),
			})
		case "Import SSO start-urls from ~/.aws/config":
			instances = importFromAwsConfig(instances)
		case "Remove an SSO start URL":
			var labels []string
			for _, i := range instances {
				labels = append(labels, i.StartUrl+" ("+i.Region+")")
			}
			index, _ := prompter.Select("Select the SSO start URL to remove", labels, func(input string, idx int) bool {
				return fuzzy.MatchFold(input, labels[idx])
			})
			instances = append(instances[:index], instances[index+1:]...)
		default:
			break editLoop
		}
	}
	return instances
}

func importFromAwsConfig(instances []SsoInstance) []SsoInstance {
	awsConfigPath := AwsConfigFilePath()
	discovered, err := ImportSsoInstancesFromAwsConfig(awsConfigPath)
	if err != nil {
		zap.S().Warnf("Could not read %s: %s", awsConfigPath, err)
		return instances
	}

	added := 0
	for _, d := range discovered {
		if containsStartUrl(instances, d.StartUrl) {
			continue
		}
		instances = append(instances, d)
		added++
	}
	zap.S().Infof("Found %d SSO start-url(s) in %s, imported %d new one(s)", len(discovered), awsConfigPath, added)
	return instances
}

func containsStartUrl(instances []SsoInstance, startUrl string) bool {
	for _, i := range instances {
		if i.StartUrl == startUrl {
			return true
		}
	}
	return false
}

func ReadConfig(filePath string) *AppConfig {

	bytes, err := os.ReadFile(filePath)
	check(err)
	appConfig := AppConfig{}
	err = yaml.Unmarshal(bytes, &appConfig)
	check(err)
	return &appConfig
}

// TryReadConfig reads the config file, tolerating a missing file by returning an empty AppConfig.
func TryReadConfig(filePath string) *AppConfig {
	if _, err := os.Stat(filePath); err != nil {
		return &AppConfig{}
	}
	return ReadConfig(filePath)
}

func writeConfig(filePath string, ac AppConfig) error {
	bytes, err := yaml.Marshal(ac)
	check(err)

	base := path.Dir(filePath)
	err = os.MkdirAll(base, 0755)
	check(err)

	err = os.WriteFile(filePath, bytes, 0755)
	check(err)

	zap.S().Infof("Config file generated: %s", filePath)

	return err
}

func ConfigFilePath() string {
	configDir, err := os.UserConfigDir()
	check(err)
	return configDir + "/go-aws-sso/config.yml"
}

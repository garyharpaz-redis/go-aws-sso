package internal

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestImportSsoInstancesFromAwsConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")

	content := `[default]
region = us-east-1

[profile plain]
aws_access_key_id = AKIA...
aws_secret_access_key = secret

[profile legacy-inline]
sso_start_url = https://legacy.awsapps.com/start
sso_region = eu-west-1
sso_account_id = 111111111111
sso_role_name = SomeRole
region = eu-west-1

[sso-session my-session]
sso_start_url = https://session.awsapps.com/start
sso_region = eu-central-1
sso_registration_scopes = sso:account:access

[profile uses-session]
sso_session = my-session
sso_account_id = 222222222222
sso_role_name = OtherRole
region = eu-central-1

[profile duplicate-of-session]
sso_start_url = https://session.awsapps.com/start
sso_region = eu-central-1
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ImportSsoInstancesFromAwsConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []SsoInstance{
		{Name: "legacy-inline", StartUrl: "https://legacy.awsapps.com/start", Region: "eu-west-1"},
		{Name: "my-session", StartUrl: "https://session.awsapps.com/start", Region: "eu-central-1"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got: %+v, want: %+v", got, want)
	}
}

func TestImportSsoInstancesFromAwsConfig_MissingFile(t *testing.T) {
	got, err := ImportSsoInstancesFromAwsConfig("/nonexistent/path/does/not/exist")
	if err != nil {
		t.Fatalf("expected no error for a missing file, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil result for a missing file, got: %+v", got)
	}
}

func TestAwsConfigFilePath(t *testing.T) {
	oldEnv, hadEnv := os.LookupEnv("AWS_CONFIG_FILE")
	defer func() {
		if hadEnv {
			os.Setenv("AWS_CONFIG_FILE", oldEnv)
		} else {
			os.Unsetenv("AWS_CONFIG_FILE")
		}
	}()

	os.Setenv("AWS_CONFIG_FILE", "/custom/path/config")
	if got := AwsConfigFilePath(); got != "/custom/path/config" {
		t.Errorf("got %q, want %q", got, "/custom/path/config")
	}

	os.Unsetenv("AWS_CONFIG_FILE")
	homeDir, _ := os.UserHomeDir()
	want := homeDir + "/.aws/config"
	if got := AwsConfigFilePath(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestContainsStartUrl(t *testing.T) {
	instances := []SsoInstance{
		{StartUrl: "https://one.awsapps.com/start"},
		{StartUrl: "https://two.awsapps.com/start"},
	}

	if !containsStartUrl(instances, "https://one.awsapps.com/start") {
		t.Error("expected true for an existing start-url")
	}
	if containsStartUrl(instances, "https://three.awsapps.com/start") {
		t.Error("expected false for a start-url that isn't present")
	}
}

package internal

import (
	"encoding/json"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/service/sso"
	"github.com/aws/aws-sdk-go/service/sso/ssoiface"
	"github.com/aws/aws-sdk-go/service/ssooidc/ssooidciface"
	. "github.com/theurichde/go-aws-sso/pkg/sso"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

// AssumeDirectly
// Directly assumes into a certain account and role, bypassing the prompt and interactive selection.
func AssumeDirectly(oidcClient ssooidciface.SSOOIDCAPI, ssoClient ssoiface.SSOAPI, context *cli.Context) {
	startUrl := context.String("start-url")
	if startUrl == "" {
		zap.S().Fatal("Multiple SSO instances are configured, so the start-url can't be determined automatically for a direct assume. " +
			"Pass -u/--start-url and -r/--region explicitly, or run `go-aws-sso` interactively once to pick an account.")
	}
	LoadRuntimeConfig(context.Bool("headless"))
	accountId := context.String("account-id")
	roleName := context.String("role-name")
	clientInformation := ProcessClientInformation(oidcClient, startUrl)
	rci := &sso.GetRoleCredentialsInput{AccountId: &accountId, RoleName: &roleName, AccessToken: &clientInformation.AccessToken}
	roleCredentials, err := ssoClient.GetRoleCredentials(rci)
	check(err)

	if context.Bool("persist") {
		template := ProcessPersistedCredentialsTemplate(roleCredentials, context.String("region"))
		WriteAWSCredentialsFile(&template, context.String("profile"))

		zap.S().Infof("Credentials expire at: %s\n", time.Unix(*roleCredentials.RoleCredentials.Expiration/1000, 0))
		PrintAssumedIdentity(context.String("profile"), "", accountId, roleName)
	} else {
		template := ProcessCredentialProcessTemplate(accountId, roleName, context.String("region"), context.String("profile"), startUrl)
		WriteAWSCredentialsFile(&template, context.String("profile"))

		creds := CredentialProcessOutput{
			Version:         1,
			AccessKeyId:     *roleCredentials.RoleCredentials.AccessKeyId,
			Expiration:      time.Unix(*roleCredentials.RoleCredentials.Expiration/1000, 0).Format(time.RFC3339),
			SecretAccessKey: *roleCredentials.RoleCredentials.SecretAccessKey,
			SessionToken:    *roleCredentials.RoleCredentials.SessionToken,
		}
		bytes, err := json.Marshal(creds)
		check(err)
		_, err = os.Stdout.Write(bytes)
		check(err)
	}

}

type CredentialProcessOutput struct {
	Version         int    `json:"Version"`
	AccessKeyId     string `json:"AccessKeyId"`
	Expiration      string `json:"Expiration"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken"`
}

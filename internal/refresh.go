package internal

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/sso"
	"github.com/aws/aws-sdk-go/service/sso/ssoiface"
	"github.com/aws/aws-sdk-go/service/ssooidc/ssooidciface"
	. "github.com/theurichde/go-aws-sso/pkg/sso"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

type LastUsageInformation struct {
	AccountId   string `json:"account_id"`
	AccountName string `json:"account_name"`
	Role        string `json:"role"`
	StartUrl    string `json:"start_url"`
	Region      string `json:"region"`
}

// RefreshCredentials refreshes short-living credentials based on the last account/role used.
// It resolves and builds its own SSO clients internally, since the relevant SSO instance
// (start-url/region) may need to come from the cached last-usage information rather than from
// a single global --region flag - this matters once more than one SSO instance is configured.
func RefreshCredentials(context *cli.Context) {
	LoadRuntimeConfig(context.Bool("headless"))

	lui, err := readUsageInformation()
	if err == nil {
		zap.S().Infof("Attempting to refresh credentials for account [%s] with role [%s]", lui.AccountId, lui.Role)
		oidcClient, ssoClient := InitClients(lui.Region)
		clientInformation := ProcessClientInformation(oidcClient, lui.StartUrl)
		getRoleCredentialsAndWrite(context, ssoClient, clientInformation, lui.AccountId, lui.AccountName, lui.Role, lui.Region)
		return
	}
	if !strings.Contains(err.Error(), "no such file") {
		check(err)
	}

	zap.S().Info("Nothing to refresh yet")

	if context.String("start-url") != "" {
		refreshColdSingleInstance(context)
		return
	}

	refreshColdMultiInstance(context)
}

// refreshColdSingleInstance handles the cold-cache case when exactly one SSO instance is
// configured (or an explicit -u/-r override was given) - behaves exactly like the tool did
// before multi-instance support existed.
func refreshColdSingleInstance(context *cli.Context) {
	startUrl := context.String("start-url")
	instance := SsoInstance{StartUrl: startUrl, Region: context.String("region")}
	oidcClient, ssoClient := InitClients(instance.Region)
	clientInformation := ProcessClientInformation(oidcClient, startUrl)

	accountInfo, awsErr := RetrieveAccountInfo(clientInformation, ssoClient, Prompter{})
	if awsErr != nil {
		if awsErr.StatusCode() == 401 { // unauthorized
			clientInformation, accountInfo = retryWithNewClientCreds(oidcClient, ssoClient, startUrl)
		} else {
			check(awsErr)
		}
	}
	roleInfo, roleErr := RetrieveRoleInfo(accountInfo, clientInformation, ssoClient, Prompter{})
	check(roleErr)

	SaveUsageInformation(accountInfo, roleInfo, instance)
	getRoleCredentialsAndWrite(context, ssoClient, clientInformation, *accountInfo.AccountId, *accountInfo.AccountName, *roleInfo.RoleName, instance.Region)
}

// refreshColdMultiInstance handles the cold-cache case when more than one SSO instance is
// configured: it runs the same merged, multi-instance account/role selection as the interactive
// root command.
func refreshColdMultiInstance(context *cli.Context) {
	instances := TryReadConfig(ConfigFilePath()).Instances()
	tagged, err := ListAllAccounts(instances)
	check(err)

	selected := SelectMergedAccount(tagged, Prompter{})
	roles, roleErr := ListRoles(&selected.AccountInfo, selected.ClientInformation, selected.SsoClient)
	check(roleErr)
	roleInfo := SelectRole(roles, Prompter{})

	SaveUsageInformation(&selected.AccountInfo, roleInfo, selected.Instance)
	getRoleCredentialsAndWrite(context, selected.SsoClient, selected.ClientInformation, *selected.AccountId, *selected.AccountName, *roleInfo.RoleName, selected.Instance.Region)
}

func getRoleCredentialsAndWrite(context *cli.Context, ssoClient ssoiface.SSOAPI, clientInformation ClientInformation, accountId string, accountName string, roleName string, region string) {
	rci := &sso.GetRoleCredentialsInput{AccountId: &accountId, RoleName: &roleName, AccessToken: &clientInformation.AccessToken}
	roleCredentials, err := ssoClient.GetRoleCredentials(rci)
	check(err)

	template := ProcessPersistedCredentialsTemplate(roleCredentials, region)
	WriteAWSCredentialsFile(&template, context.String("profile"))

	zap.S().Infof("Credentials expire at: %s\n", time.Unix(*roleCredentials.RoleCredentials.Expiration/1000, 0))
	PrintAssumedIdentity(context.String("profile"), accountName, accountId, roleName)
}

func retryWithNewClientCreds(oidcClient ssooidciface.SSOOIDCAPI, ssoClient ssoiface.SSOAPI, startUrl string) (ClientInformation, *sso.AccountInfo) {
	osErr := os.Remove(ClientInfoFileDestination(startUrl))
	check(osErr)
	clientInformation := ProcessClientInformation(oidcClient, startUrl)
	accountInfo, awsErr := RetrieveAccountInfo(clientInformation, ssoClient, Prompter{})
	check(awsErr)
	return clientInformation, accountInfo
}

func SaveUsageInformation(accountInfo *sso.AccountInfo, roleInfo *sso.RoleInfo, instance SsoInstance) {
	homeDir, _ := os.UserHomeDir()
	target := homeDir + "/.aws/sso/cache/last-usage.json"
	usageInformation := LastUsageInformation{
		AccountId:   *accountInfo.AccountId,
		AccountName: *accountInfo.AccountName,
		Role:        *roleInfo.RoleName,
		StartUrl:    instance.StartUrl,
		Region:      instance.Region,
	}
	WriteStructToFile(usageInformation, target)
}

func readUsageInformation() (*LastUsageInformation, error) {
	homeDir, _ := os.UserHomeDir()
	bytes, err := os.ReadFile(homeDir + "/.aws/sso/cache/last-usage.json")
	if err != nil {
		return nil, err
	}
	lui := new(LastUsageInformation)
	err = json.Unmarshal(bytes, lui)
	check(err)
	return lui, nil
}

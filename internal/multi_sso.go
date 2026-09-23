package internal

import (
	"sort"
	"strconv"

	"github.com/aws/aws-sdk-go/service/sso"
	"github.com/aws/aws-sdk-go/service/sso/ssoiface"
	. "github.com/theurichde/go-aws-sso/pkg/sso"
	"go.uber.org/zap"
)

// TaggedAccount pairs an SSO account with the SSO instance (start-url/region) and clients it was
// discovered through, so that once an account is selected from a merged, multi-instance list, the
// tool still knows exactly which organization to talk to for listing/assuming its roles.
type TaggedAccount struct {
	sso.AccountInfo
	Instance          SsoInstance
	ClientInformation ClientInformation
	SsoClient         ssoiface.SSOAPI
}

// ListAllAccounts authenticates against every configured SSO instance (triggering a device
// authorization / browser verification for each one that isn't already cached) and returns all
// of their accounts merged into a single list, sorted by account name.
func ListAllAccounts(instances []SsoInstance) ([]TaggedAccount, error) {
	var perInstance [][]TaggedAccount

	for i, instance := range instances {
		zap.S().Infof("Authenticating with SSO instance %d/%d (%s)", i+1, len(instances), instance.StartUrl)

		oidcClient, ssoClient := InitClients(instance.Region)
		clientInformation := ProcessClientInformation(oidcClient, instance.StartUrl)

		accounts, err := ListAccounts(clientInformation, ssoClient)
		if err != nil {
			return nil, err
		}

		var tagged []TaggedAccount
		for _, account := range accounts {
			tagged = append(tagged, TaggedAccount{
				AccountInfo:       account,
				Instance:          instance,
				ClientInformation: clientInformation,
				SsoClient:         ssoClient,
			})
		}
		perInstance = append(perInstance, tagged)
	}

	return mergeAccounts(perInstance), nil
}

// mergeAccounts flattens the per-instance account listings into a single list, sorted by
// account name. Split out from ListAllAccounts so the merge/sort behavior can be unit tested
// without needing real, authenticated SSO clients.
func mergeAccounts(perInstance [][]TaggedAccount) []TaggedAccount {
	var tagged []TaggedAccount
	for _, accounts := range perInstance {
		tagged = append(tagged, accounts...)
	}

	sort.Slice(tagged, func(i, j int) bool {
		return *tagged[i].AccountName < *tagged[j].AccountName
	})

	return tagged
}

// SelectMergedAccount prompts the user to interactively pick one account from a merged,
// multi-instance list. The picker display is identical to the single-instance one - the source
// SSO instance is deliberately not shown, since once configured it isn't meant to matter anymore.
func SelectMergedAccount(accounts []TaggedAccount, selector Prompt) *TaggedAccount {
	var accountsToSelect []string
	linePrefix := "#"

	for i, info := range accounts {
		accountsToSelect = append(accountsToSelect, linePrefix+strconv.Itoa(i)+" "+*info.AccountName+" "+*info.AccountId)
	}

	zap.S().Info("Hint: fuzzy search supported. To choose one account directly just enter #{Int}.")

	indexChoice, _ := selector.Select("Select your account", accountsToSelect, fuzzySearchWithPrefixAnchor(accountsToSelect, linePrefix))

	account := accounts[indexChoice]
	zap.S().Infof("Selected account: %s - %s", *account.AccountName, *account.AccountId)
	return &account
}

package internal

import (
	"fmt"
	"sort"

	. "github.com/theurichde/go-aws-sso/pkg/sso"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

// ListAccountsAndRoles prints every account (and its available roles) across the resolved SSO
// instance(s), without fetching or writing any credentials. If -u/--start-url is explicitly
// given, only that one instance is listed; otherwise every configured SSO instance is.
func ListAccountsAndRoles(context *cli.Context) {
	LoadRuntimeConfig(context.Bool("headless"))

	var instances []SsoInstance
	if context.String("start-url") != "" {
		instances = []SsoInstance{{StartUrl: context.String("start-url"), Region: context.String("region")}}
	} else {
		instances = TryReadConfig(ConfigFilePath()).Instances()
	}

	if len(instances) == 0 {
		zap.S().Fatal("No SSO start-url configured. Run `go-aws-sso config generate` first, or pass -u/--start-url and -r/--region.")
	}

	accounts, err := ListAllAccounts(instances)
	check(err)

	if len(accounts) == 0 {
		zap.S().Info("No accounts available.")
		return
	}

	for i, account := range accounts {
		roles, roleErr := ListRoles(&account.AccountInfo, account.ClientInformation, account.SsoClient)
		check(roleErr)

		roleNames := make([]string, 0, len(roles))
		for _, role := range roles {
			roleNames = append(roleNames, *role.RoleName)
		}
		sort.Strings(roleNames)

		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%s (%s)\n", *account.AccountName, *account.AccountId)
		for _, name := range roleNames {
			fmt.Printf("  - %s\n", name)
		}
	}
}

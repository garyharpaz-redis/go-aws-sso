package internal

import (
	"go.uber.org/zap"
)

// PrintAssumedIdentity logs a clear, final confirmation of exactly which profile, account and
// role were assumed. Uses zap exclusively (never fmt/stdout directly) so it is correctly
// silenced by --quiet - this matters because --quiet is also used when this tool is re-invoked
// as a credential_process, whose stdout must contain nothing but the raw JSON credentials.
func PrintAssumedIdentity(profile string, accountName string, accountId string, roleName string) {
	zap.S().Infof("Profile: %s", profile)
	if accountName != "" {
		zap.S().Infof("Account: %s (%s)", accountName, accountId)
	} else {
		zap.S().Infof("Account: %s", accountId)
	}
	zap.S().Infof("Role:    %s", roleName)
}

package main

import (
	"flag"
	"os"
	"testing"

	ssolib "github.com/theurichde/go-aws-sso/pkg/sso"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestForceCannotInvalidateCacheDuringAuthorization(t *testing.T) {
	release, err := ssolib.AcquireAuthorizationLock()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	startURL := "https://force-lock-test.invalid/start"
	cache := ssolib.ClientInfoFileDestination(startURL)
	if err := os.WriteFile(cache, []byte("cached-session"), 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(cache)

	restore := zap.ReplaceGlobals(zap.New(zapcore.NewNopCore(), zap.WithFatalHook(zapcore.WriteThenPanic)))
	defer restore()
	flags := flag.NewFlagSet("force", flag.ContinueOnError)
	flags.Bool("force", true, "")
	flags.String("start-url", startURL, "")
	ctx := cli.NewContext(nil, flags, nil)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("force should fail while authorization is active")
			}
		}()
		applyForceFlag(ctx)
	}()
	content, err := os.ReadFile(cache)
	if err != nil || string(content) != "cached-session" {
		t.Fatalf("force changed a cache protected by active authorization: %q, %v", content, err)
	}
	release()
	applyForceFlag(ctx)
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("force failed to remove cache after lock release: %v", err)
	}
}

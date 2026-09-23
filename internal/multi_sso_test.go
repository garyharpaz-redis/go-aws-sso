package internal

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/sso"
)

func taggedAccount(name string, id string, instance SsoInstance) TaggedAccount {
	n, i := name, id
	return TaggedAccount{
		AccountInfo: sso.AccountInfo{AccountName: &n, AccountId: &i},
		Instance:    instance,
	}
}

func TestMergeAccounts(t *testing.T) {
	instanceA := SsoInstance{StartUrl: "https://a.awsapps.com/start", Region: "eu-central-1"}
	instanceB := SsoInstance{StartUrl: "https://b.awsapps.com/start", Region: "us-east-1"}

	perInstance := [][]TaggedAccount{
		{
			taggedAccount("Zebra", "111", instanceA),
			taggedAccount("Apple", "222", instanceA),
		},
		{
			taggedAccount("Mango", "333", instanceB),
		},
	}

	got := mergeAccounts(perInstance)

	if len(got) != 3 {
		t.Fatalf("got %d accounts, want 3", len(got))
	}

	wantOrder := []string{"Apple", "Mango", "Zebra"}
	for i, want := range wantOrder {
		if *got[i].AccountName != want {
			t.Errorf("position %d: got %q, want %q", i, *got[i].AccountName, want)
		}
	}

	// the account tagged from instance B must still carry instance B's start-url, not instance A's.
	for _, a := range got {
		if *a.AccountName == "Mango" && a.Instance.StartUrl != instanceB.StartUrl {
			t.Errorf("Mango tagged with start-url %q, want %q", a.Instance.StartUrl, instanceB.StartUrl)
		}
		if *a.AccountName != "Mango" && a.Instance.StartUrl != instanceA.StartUrl {
			t.Errorf("%s tagged with start-url %q, want %q", *a.AccountName, a.Instance.StartUrl, instanceA.StartUrl)
		}
	}
}

func TestSelectMergedAccount(t *testing.T) {
	instanceA := SsoInstance{StartUrl: "https://a.awsapps.com/start", Region: "eu-central-1"}
	instanceB := SsoInstance{StartUrl: "https://b.awsapps.com/start", Region: "us-east-1"}

	accounts := []TaggedAccount{
		taggedAccount("Apple", "222", instanceA),
		taggedAccount("Mango", "333", instanceB),
	}

	selected := SelectMergedAccount(accounts, mockPromptUISelector{index: 1})

	if *selected.AccountName != "Mango" {
		t.Errorf("got account %q, want Mango", *selected.AccountName)
	}
	if selected.Instance.StartUrl != instanceB.StartUrl {
		t.Errorf("got instance %q, want %q", selected.Instance.StartUrl, instanceB.StartUrl)
	}
}

type mockPromptUISelector struct {
	index int
}

func (m mockPromptUISelector) Select(_ string, _ []string, _ func(input string, index int) bool) (int, string) {
	return m.index, ""
}

func (m mockPromptUISelector) Prompt(_ string, _ string) string {
	return ""
}

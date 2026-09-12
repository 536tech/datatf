package inventory

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/databricks/databricks-sdk-go/service/catalog"
)

func TestRedactStorageCredentialDropsSecrets(t *testing.T) {
	info := catalog.StorageCredentialInfo{
		Name: "adls",
		AzureServicePrincipal: &catalog.AzureServicePrincipal{
			ApplicationId: "app", DirectoryId: "dir", ClientSecret: "canary-secret",
		},
		CloudflareApiToken: &catalog.CloudflareApiToken{
			AccessKeyId: "key", AccountId: "acct", SecretAccessKey: "canary-token",
		},
	}
	original, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	redacted, err := json.Marshal(redactStorageCredential(info))
	if err != nil {
		t.Fatal(err)
	}
	for _, canary := range []string{"canary-secret", "canary-token"} {
		if strings.Contains(string(redacted), canary) {
			t.Fatalf("secret survived redaction: %s", redacted)
		}
	}
	for _, kept := range []string{`"application_id":"app"`, `"directory_id":"dir"`, `"access_key_id":"key"`} {
		if !strings.Contains(string(redacted), kept) {
			t.Fatalf("non-secret field dropped: %s", redacted)
		}
	}
	if !strings.Contains(string(original), "canary-secret") {
		t.Fatal("redaction mutated the caller's value")
	}
}

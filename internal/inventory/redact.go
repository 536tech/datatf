package inventory

import "github.com/databricks/databricks-sdk-go/service/catalog"

// redactStorageCredential drops credential secret fields before the SDK struct is
// serialized into inventory files or JSON output. Unity Catalog redacts these values on
// read today; DataTF never relies on that.
func redactStorageCredential(info catalog.StorageCredentialInfo) catalog.StorageCredentialInfo {
	if info.AzureServicePrincipal != nil {
		sp := *info.AzureServicePrincipal
		sp.ClientSecret = ""
		info.AzureServicePrincipal = &sp
	}
	if info.CloudflareApiToken != nil {
		token := *info.CloudflareApiToken
		token.SecretAccessKey = ""
		info.CloudflareApiToken = &token
	}
	return info
}

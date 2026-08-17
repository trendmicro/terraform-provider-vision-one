package config

const (
	RESOURCE_TYPE_AZURE_UDC_EVENTHUB_INFO = "clm_udc_eventhub_info"

	RESOURCE_TYPE_AZURE_UDC_EVENTHUB_INFO_DESCRIPTION = "The `" + RESOURCE_TYPE_AZURE_UDC_EVENTHUB_INFO + "` resource registers a subscription's Azure Event Hub stack as a log source for Trend Vision One Cloud Log Monitoring. One record covers the whole stack — the resource group, every namespace, and the hubs and consumer groups inside them — so Vision One knows what to read for that subscription. Adding a vendor updates this record rather than creating another. Terraform cannot verify this record after registration — CLM's read API for it is routed internally, and Vision One's gateway blocks device tokens from internal routing by policy regardless of network access, so a later plan won't detect a write that silently failed or was changed by another caller."
)

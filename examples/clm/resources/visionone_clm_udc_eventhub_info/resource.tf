resource "visionone_clm_udc_eventhub_info" "example" {
  tenant_id        = "00000000-0000-0000-0000-000000000001"
  subscription_id  = "00000000-0000-0000-0000-000000000002"
  region           = "eastus"
  resource_group   = "trendai-clm-udc-eh-rg"
  scm_device_token = var.scm_device_token

  event_hub_namespaces = [
    {
      name = "trendai-clm-udc-eh-ns-0"
      event_hubs = [
        {
          name            = "trendai-example-vendor-logs"
          consumer_groups = ["trendai-example-vendor-cg-0"]
        }
      ]
    }
  ]
}

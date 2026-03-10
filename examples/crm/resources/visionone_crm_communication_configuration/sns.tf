resource "visionone_crm_communication_configuration" "amazon_sns" {
  enabled       = true
  channel_label = "AWS Event Stream"
  manual        = true

  sns_configuration = {
    arn = "arn:aws:sns:us-east-1:123456789012:cloud-security-events"
  }

  checks_filter = {
    statuses = ["SUCCESS", "FAILURE"]
    regions  = ["us-east-1", "eu-west-1", "ap-southeast-2"]
  }
}
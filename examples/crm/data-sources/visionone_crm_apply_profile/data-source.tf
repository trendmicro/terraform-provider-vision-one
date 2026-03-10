data "visionone_crm_apply_profile" "apply_profile" {
  profile_id  = "<profile_id>"
  account_ids = ["<account_id_1>", "<account_id_2>"]
  mode        = "overwrite" # fill-gaps | overwrite | replace
  notes       = "Applying profile via Terraform"

  include {
    exceptions = false
  }
}
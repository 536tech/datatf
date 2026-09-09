mock_provider "databricks" {}

run "exported_root" {
  command = plan
}

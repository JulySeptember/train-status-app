output "tfstate_bucket_name" {
  value = aws_s3_bucket.tfstate.bucket
}

output "artifact_bucket_name" {
  value = aws_s3_bucket.artifact.bucket
}

output "terraform_lock_table" {
  value = aws_dynamodb_table.terraform_lock.name
}

output "github_plan_role_arn" {
  value = aws_iam_role.github_plan.arn
}

output "github_deploy_role_arn" {
  value = aws_iam_role.github_deploy.arn
}

output "tfstate_bucket_name" {
  value = aws_s3_bucket.tfstate.bucket
}

output "artifact_bucket_name" {
  value = aws_s3_bucket.artifact.bucket
}

output "terraform_lock_table" {
  value = aws_dynamodb_table.terraform_lock.name
}

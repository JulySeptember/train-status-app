output "cloudfront_domain_name" {
  value = aws_cloudfront_distribution.this.domain_name
}

output "cloudfront_distribution_id" {
  value = aws_cloudfront_distribution.this.id
}

output "frontend_bucket_name" {
  value = aws_s3_bucket.frontend.bucket
}

output "lambda_function_name" {
  value = aws_lambda_function.this.function_name
}

output "api_endpoint" {
  value = aws_apigatewayv2_api.this.api_endpoint
}

data "aws_cloudfront_cache_policy" "caching_optimized" {
  name = "Managed-CachingOptimized"
}

data "aws_cloudfront_cache_policy" "caching_disabled" {
  name = "Managed-CachingDisabled"
}

data "aws_cloudfront_origin_request_policy" "all_viewer_except_host_header" {
  name = "Managed-AllViewerExceptHostHeader"
}

# Managed-SecurityHeadersPolicy と同じ内容に、CSP と Permissions-Policy を足したもの。
# AI の応答を画面に出し、セッション Cookie を使うので、XSS が起きたときの被害を抑える
locals {
  # Google でのログイン（frontend/src/lib/auth.ts の Google Identity Services）に要るものは、
  # https://developers.google.com/identity/gsi/web/guides/get-google-api-clientid#content_security_policy に合わせる。
  # style-src の 'unsafe-inline' は、Google のボタン（gsi/client）がアカウントの画面で当てるインラインのスタイルのため
  # （外すと違反になるのを確かめた）。スクリプトのインラインは許さない
  content_security_policy = join("; ", [
    "default-src 'self'",
    "script-src 'self' https://accounts.google.com/gsi/client",
    "style-src 'self' 'unsafe-inline' https://accounts.google.com/gsi/style",
    "connect-src 'self' https://accounts.google.com/gsi/",
    "frame-src https://accounts.google.com/gsi/",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'self'",
  ])

  # 位置情報は「近くの駅」（frontend/src/components/NearbyStations.tsx）で使う
  permissions_policy = "geolocation=(self), camera=(), microphone=(), payment=(), usb=()"
}

resource "aws_cloudfront_response_headers_policy" "security_headers" {
  name    = "${local.name_prefix}-security-headers"
  comment = "Managed-SecurityHeadersPolicy + CSP + Permissions-Policy"

  security_headers_config {
    strict_transport_security {
      access_control_max_age_sec = 31536000
      override                   = false
    }

    content_type_options {
      override = true
    }

    frame_options {
      frame_option = "SAMEORIGIN"
      override     = false
    }

    referrer_policy {
      referrer_policy = "strict-origin-when-cross-origin"
      override        = false
    }

    xss_protection {
      protection = true
      mode_block = true
      override   = false
    }

    # 強制するときだけここで付ける（Report-Only はカスタムヘッダーで付ける）
    dynamic "content_security_policy" {
      for_each = var.csp_report_only ? [] : [1]
      content {
        content_security_policy = local.content_security_policy
        override                = true
      }
    }
  }

  custom_headers_config {
    items {
      header   = "Permissions-Policy"
      value    = local.permissions_policy
      override = true
    }

    dynamic "items" {
      for_each = var.csp_report_only ? [1] : []
      content {
        header   = "Content-Security-Policy-Report-Only"
        value    = local.content_security_policy
        override = true
      }
    }
  }
}

resource "aws_cloudfront_origin_access_control" "frontend" {
  name                              = "${local.name_prefix}-frontend-oac"
  description                       = "Origin Access Control for Frontend S3"
  origin_access_control_origin_type = "s3"

  signing_behavior = "always"
  signing_protocol = "sigv4"
}

# API（/api/*）のエラーを index.html で上書きしないよう、custom_error_response ではなく
# S3 向けの振る舞いだけに関数を付けて SPA のルーティングに対応する
resource "aws_cloudfront_function" "spa_rewrite" {
  name    = "${local.name_prefix}-spa-rewrite"
  runtime = "cloudfront-js-2.0"
  comment = "Rewrite SPA routes to /index.html"
  publish = true
  code    = file("${path.module}/functions/spa_rewrite.js")
}

resource "aws_cloudfront_distribution" "this" {

  enabled             = true
  wait_for_deployment = false

  comment = "${local.name_prefix} CloudFront"

  default_root_object = "index.html"

  is_ipv6_enabled = true
  http_version    = "http2and3"

  price_class = "PriceClass_100"

  origin {
    origin_id   = "frontend"
    domain_name = aws_s3_bucket.frontend.bucket_regional_domain_name

    origin_access_control_id = aws_cloudfront_origin_access_control.frontend.id
  }

  origin {
    origin_id = "api"

    domain_name = replace(
      aws_apigatewayv2_api.this.api_endpoint,
      "https://",
      ""
    )

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"

      origin_ssl_protocols = [
        "TLSv1.2"
      ]
    }
  }

  default_cache_behavior {

    target_origin_id       = "frontend"
    viewer_protocol_policy = "redirect-to-https"

    allowed_methods = [
      "GET",
      "HEAD"
    ]

    cached_methods = [
      "GET",
      "HEAD"
    ]

    compress = true

    cache_policy_id            = data.aws_cloudfront_cache_policy.caching_optimized.id
    response_headers_policy_id = aws_cloudfront_response_headers_policy.security_headers.id

    function_association {
      event_type   = "viewer-request"
      function_arn = aws_cloudfront_function.spa_rewrite.arn
    }
  }

  ordered_cache_behavior {

    path_pattern = "/api/*"

    target_origin_id       = "api"
    viewer_protocol_policy = "redirect-to-https"

    allowed_methods = [
      "GET",
      "HEAD",
      "OPTIONS",
      "PUT",
      "POST",
      "PATCH",
      "DELETE"
    ]

    cached_methods = [
      "GET",
      "HEAD"
    ]

    compress = true

    cache_policy_id            = data.aws_cloudfront_cache_policy.caching_disabled.id
    origin_request_policy_id   = data.aws_cloudfront_origin_request_policy.all_viewer_except_host_header.id
    response_headers_policy_id = aws_cloudfront_response_headers_policy.security_headers.id
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }

  tags = local.common_tags
}

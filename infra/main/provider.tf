terraform {
  required_version = ">= 1.12.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  backend "s3" {
    bucket         = "train-status-app-dev-terraform-state"
    key            = "main/terraform.tfstate"
    region         = "ap-northeast-1"
    dynamodb_table = "train-status-app-dev-terraform-lock"
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = var.tags
  }
}

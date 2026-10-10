terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.60"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }

  # 每个环境使用独立 workspace，避免 dev 和 prod 共用一份 state。
  # 示例：terraform workspace new dev && terraform apply -var-file=envs/dev.tfvars
  backend "local" {}
}

provider "aws" {
  region = var.region
}

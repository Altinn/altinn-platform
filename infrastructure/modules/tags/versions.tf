terraform {
  required_version = ">= 1.0"

  required_providers {
    time = {
      source  = "hashicorp/time"
      version = "~> 0.9"
    }
    http = {
      source  = "hashicorp/http"
      version = "~> 3.4"
    }
  }
}

provider "aws" {
  region = "ap-southeast-1"
}

terraform {
	required_providers {
		aws = {
	    version = "~> 5.100.0"
		}
  }
}

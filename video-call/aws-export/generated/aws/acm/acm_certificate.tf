resource "aws_acm_certificate" "tfer--128babff-071f-46e1-8a5f-f4dd30371009_ninh-video-call-demo-002E-food" {
  domain_name   = "ninh-video-call-demo.food"
  key_algorithm = "RSA_2048"

  options {
    certificate_transparency_logging_preference = "ENABLED"
  }

  subject_alternative_names = ["*.ninh-video-call-demo.food", "ninh-video-call-demo.food"]
  validation_method         = "DNS"
}

resource "aws_acm_certificate" "tfer--cabf80d0-6e36-4867-997d-82fd10a4f284_ninh-video-call-demo-002E-food" {
  domain_name   = "ninh-video-call-demo.food"
  key_algorithm = "RSA_2048"

  options {
    certificate_transparency_logging_preference = "ENABLED"
  }

  subject_alternative_names = ["*.ninh-video-call-demo.food", "ninh-video-call-demo.food"]
  validation_method         = "DNS"
}

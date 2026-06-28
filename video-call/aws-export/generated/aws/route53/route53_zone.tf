resource "aws_route53_zone" "tfer--Z047764539BW6DC9FC5O5_u2u-002E-xyz" {
  force_destroy = "false"
  name          = "u2u.xyz"
}

resource "aws_route53_zone" "tfer--Z05145943JAIPN98TAEII_ninh-video-call-demo-002E-food" {
  comment       = "video call app route 53 domain"
  force_destroy = "false"
  name          = "ninh-video-call-demo.food"
}

resource "aws_route53_record" "tfer--Z047764539BW6DC9FC5O5_u2u-002E-xyz-002E-_NS_" {
  multivalue_answer_routing_policy = "false"
  name                             = "u2u.xyz"
  records                          = ["ns-1454.awsdns-53.org.", "ns-1720.awsdns-23.co.uk.", "ns-37.awsdns-04.com.", "ns-784.awsdns-34.net."]
  ttl                              = "172800"
  type                             = "NS"
  zone_id                          = "${aws_route53_zone.tfer--Z047764539BW6DC9FC5O5_u2u-002E-xyz.zone_id}"
}

resource "aws_route53_record" "tfer--Z047764539BW6DC9FC5O5_u2u-002E-xyz-002E-_SOA_" {
  multivalue_answer_routing_policy = "false"
  name                             = "u2u.xyz"
  records                          = ["ns-1720.awsdns-23.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl                              = "900"
  type                             = "SOA"
  zone_id                          = "${aws_route53_zone.tfer--Z047764539BW6DC9FC5O5_u2u-002E-xyz.zone_id}"
}

resource "aws_route53_record" "tfer--Z05145943JAIPN98TAEII__5461c3d3bd1e6d5773debd439a537728-002E-ninh-video-call-demo-002E-food-002E-_CNAME_" {
  multivalue_answer_routing_policy = "false"
  name                             = "_5461c3d3bd1e6d5773debd439a537728.ninh-video-call-demo.food"
  records                          = ["_714fb4819db488fecf2cd2d3b5fd0dcb.jkddzztszm.acm-validations.aws."]
  ttl                              = "300"
  type                             = "CNAME"
  zone_id                          = "${aws_route53_zone.tfer--Z05145943JAIPN98TAEII_ninh-video-call-demo-002E-food.zone_id}"
}

resource "aws_route53_record" "tfer--Z05145943JAIPN98TAEII_ninh-video-call-demo-002E-food-002E-_A_" {
  alias {
    evaluate_target_health = "true"
    name                   = "dualstack.videocall-alb-1920905937.ap-southeast-1.elb.amazonaws.com"
    zone_id                = "Z1LMS91P8CMLE5"
  }

  multivalue_answer_routing_policy = "false"
  name                             = "ninh-video-call-demo.food"
  type                             = "A"
  zone_id                          = "${aws_route53_zone.tfer--Z05145943JAIPN98TAEII_ninh-video-call-demo-002E-food.zone_id}"
}

resource "aws_route53_record" "tfer--Z05145943JAIPN98TAEII_ninh-video-call-demo-002E-food-002E-_NS_" {
  multivalue_answer_routing_policy = "false"
  name                             = "ninh-video-call-demo.food"
  records                          = ["ns-1038.awsdns-01.org.", "ns-2037.awsdns-62.co.uk.", "ns-57.awsdns-07.com.", "ns-987.awsdns-59.net."]
  ttl                              = "172800"
  type                             = "NS"
  zone_id                          = "${aws_route53_zone.tfer--Z05145943JAIPN98TAEII_ninh-video-call-demo-002E-food.zone_id}"
}

resource "aws_route53_record" "tfer--Z05145943JAIPN98TAEII_ninh-video-call-demo-002E-food-002E-_SOA_" {
  multivalue_answer_routing_policy = "false"
  name                             = "ninh-video-call-demo.food"
  records                          = ["ns-1038.awsdns-01.org. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl                              = "900"
  type                             = "SOA"
  zone_id                          = "${aws_route53_zone.tfer--Z05145943JAIPN98TAEII_ninh-video-call-demo-002E-food.zone_id}"
}

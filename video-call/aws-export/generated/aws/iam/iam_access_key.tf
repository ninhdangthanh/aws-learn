resource "aws_iam_access_key" "tfer--AKIAZ3OCR6GGEQCRISP6" {
  depends_on = ["aws_iam_user.tfer--AIDAZ3OCR6GGMBCUZF7BU"]
  status     = "Active"
  user       = "ninh-mac-air-m4"
}

resource "aws_iam_access_key" "tfer--AKIAZ3OCR6GGHJU45VZK" {
  depends_on = ["aws_iam_user.tfer--AIDAZ3OCR6GGDGS5VOAJO"]
  status     = "Active"
  user       = "github-actions-videocall-deploy"
}

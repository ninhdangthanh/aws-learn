resource "aws_iam_user" "tfer--AIDAZ3OCR6GGDGS5VOAJO" {
  force_destroy = "false"
  name          = "github-actions-videocall-deploy"
  path          = "/"
}

resource "aws_iam_user" "tfer--AIDAZ3OCR6GGMBCUZF7BU" {
  force_destroy        = "false"
  name                 = "ninh-mac-air-m4"
  path                 = "/"
  permissions_boundary = "arn:aws:iam::aws:policy/AdministratorAccess"

  tags = {
    AKIAZ3OCR6GGEQCRISP6 = "local development"
  }

  tags_all = {
    AKIAZ3OCR6GGEQCRISP6 = "local development"
  }
}

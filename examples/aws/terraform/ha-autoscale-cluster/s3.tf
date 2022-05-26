// S3 bucket is used to distribute letsencrypt certificates
resource "aws_s3_bucket" "certs" {
  bucket        = var.s3_bucket_name
  force_destroy = true
}

resource "aws_s3_bucket_acl" "certs" {
  bucket = aws_s3_bucket.certs.bucket
  acl    = "private"
}

resource "aws_s3_bucket_server_side_encryption_configuration" "certs" {
  bucket = aws_s3_bucket.certs.bucket

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_object" "grafana_teleport_dashboard" {
  bucket = aws_s3_bucket.certs.bucket
  key    = "health-dashboard.json"
  source = "./assets/health-dashboard.json"
}

// Grafana nginx config (letsencrypt)
resource "aws_s3_object" "grafana_teleport_nginx" {
  bucket = aws_s3_bucket.certs.bucket
  key    = "grafana-nginx.conf"
  source = "./assets/grafana-nginx.conf"
  count  = var.use_acm ? 0 : 1
}

// Grafana nginx config (ACM)
resource "aws_s3_object" "grafana_teleport_nginx_acm" {
  bucket = aws_s3_bucket.certs.bucket
  key    = "grafana-nginx.conf"
  source = "./assets/grafana-nginx-acm.conf"
  count  = var.use_acm ? 1 : 0
}

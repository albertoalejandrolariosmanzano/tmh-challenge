// Recurso S3 para almacenar el estado de Terraform
resource "aws_s3_bucket" "terraform_state" {
  bucket = var.tf_state_bucket

  tags = {
    Name = "terraform-state-${var.cluster_name}"
  }
}

// Habilitar versionado del bucket mediante el recurso dedicado (reemplaza el bloque versioning deprecado)
resource "aws_s3_bucket_versioning" "terraform_state_versioning" {
  bucket = aws_s3_bucket.terraform_state.id

  versioning_configuration {
    status = "Enabled"
  }
}

// Recomendado: bloquear bucket contra eliminación accidental
resource "aws_s3_bucket_public_access_block" "terraform_state_block" {
  bucket = aws_s3_bucket.terraform_state.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

// Opcional: habilitar cifrado a nivel de bucket con KMS (no incluido por defecto)

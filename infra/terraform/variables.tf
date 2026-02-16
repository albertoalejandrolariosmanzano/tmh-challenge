// Variables para terraform (estructura)

variable "aws_region" {
  description = "AWS region to deploy resources in"
  type        = string
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
}

variable "public_subnet_cidr" {
  description = "CIDR block for the public subnet"
  type        = string
}

variable "db_username" {
  description = "Master username for the RDS instance"
  type        = string
}

variable "db_password" {
  description = "Master password for the RDS instance (sensitive)"
  type        = string
  sensitive   = true
}

variable "db_instance_class" {
  description = "Instance class for RDS"
  type        = string
}

variable "db_allocated_storage" {
  description = "Allocated storage (GB) for RDS"
  type        = number
}

variable "cluster_name" {
  description = "Name for EKS cluster"
  type        = string
}

variable "cluster_version" {
  description = "Kubernetes version for the EKS cluster"
  type        = string
}

variable "cluster_class" {
  description = "Instance class for EKS nodes"
  type        = string
}

variable "tf_state_bucket" {
  description = "S3 bucket name to store Terraform state (backend)"
  type        = string
}

variable "tf_state_key" {
  description = "Object key (path) inside the S3 bucket for the state file"
  type        = string
}

variable "tf_state_region" {
  description = "Region where the S3 backend bucket lives"
  type        = string
}

variable "aws_availability_zone" {
  description = "Availability zone for the public subnet"
  type        = string
}
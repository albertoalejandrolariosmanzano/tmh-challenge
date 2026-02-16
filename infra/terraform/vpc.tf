# VPC
resource "aws_vpc" "tmh_vpc" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  
  tags = {
    Name = "tmh-vpc"
  }
}

# Subnets públicas
resource "aws_subnet" "public_1" {
  vpc_id            = aws_vpc.tmh_vpc.id
  cidr_block        = var.public_subnet_cidr
  availability_zone = var.aws_availability_zone
  
  tags = {
    Name = "tmh-public-1"
  }
}
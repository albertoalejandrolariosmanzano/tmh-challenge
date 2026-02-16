# RDS PostgreSQL con réplicas de lectura
resource "aws_db_instance" "postgres_primary" {
  identifier           = "tmh-postgres-primary"
  engine               = "postgres"
  engine_version       = "14.7"
  instance_class       = var.db_instance_class
  allocated_storage    = var.db_allocated_storage
  storage_type         = "gp3"
  
  username = var.db_username
  password = var.db_password
  
  vpc_security_group_ids = [aws_security_group.db_sg.id]
  db_subnet_group_name   = aws_db_subnet_group.postgres.name
  
  backup_retention_period = 7
  multi_az               = true
  
  tags = {
    Name = "tmh-postgres-primary"
  }
}

resource "aws_db_instance" "postgres_replica" {
  identifier             = "tmh-postgres-replica"
  replicate_source_db    = aws_db_instance.postgres_primary.identifier
  instance_class         = var.db_instance_class
  
  tags = {
    Name = "tmh-postgres-replica"
  }
}
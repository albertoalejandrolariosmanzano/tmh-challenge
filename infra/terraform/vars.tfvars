// Valores reales para variables (no subir a VCS si contienen secretos)

aws_region          = "us-east-1"
vpc_cidr            = "10.0.0.0/16"
public_subnet_cidr  = "10.0.1.0/24"
db_username         = "tmh_admin"
# Cambia la contraseña a una segura antes de aplicar
db_password         = "replace_with_secure_password"
db_instance_class   = "db.t3.medium"
db_allocated_storage = 100
cluster_name        = "tmh-cluster"
cluster_version     = "1.28"
cluster_class        = "t3.medium"
output "cluster_endpoint" {
  value = aws_eks_cluster.tmh_cluster.endpoint
}

output "db_endpoint" {
  value = aws_db_instance.postgres_primary.endpoint
}
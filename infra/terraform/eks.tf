resource "aws_eks_cluster" "tmh_cluster" {
  # count    = var.resource_type == "kubernetes" && var.action != "delete" ? 1 : 0
  name     = var.cluster_name
  role_arn = aws_iam_role.eks_cluster_role[0].arn
  version  = var.cluster_version
  
  vpc_config {
    subnet_ids = aws_subnet.public_1[*].id
  }
  
  depends_on = [
    aws_iam_role_policy_attachment.eks_cluster_policy,
  ]
  
  tags = {
    Name = "${var.cluster_name}-eks"
  }
}

resource "aws_eks_node_group" "tmh_node_group" {
  # count           = var.resource_type == "kubernetes" && var.action != "delete" ? 1 : 0
  cluster_name    = aws_eks_cluster.tmh_cluster[0].name
  node_group_name = "${var.cluster_name}-eks-nodes"
  node_role_arn   = aws_iam_role.eks_node_role[0].arn
  subnet_ids      = aws_subnet.public_1[*].id
  
  scaling_config {
    desired_size = 1
    max_size     = 5
    min_size     = 1
  }
  
  instance_types = [var.cluster_class]
  
  depends_on = [
    aws_iam_role_policy_attachment.eks_worker_node_policy,
    aws_iam_role_policy_attachment.eks_cni_policy,
    aws_iam_role_policy_attachment.eks_container_registry_policy,
  ]
  
  tags = {
    Name = "${var.cluster_name}-eks-nodes"
  }
}

# IAM Roles for EKS
resource "aws_iam_role" "eks_cluster_role" {
  name  = "${var.cluster_name}-eks-cluster-role"
  
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "eks.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "eks_cluster_policy" {
  # count      = var.resource_type == "kubernetes" && var.action != "delete" ? 1 : 0
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
  role       = aws_iam_role.eks_cluster_role[0].name
}

resource "aws_iam_role" "eks_node_role" {
  name  = "${var.cluster_name}-eks-node-role"
  
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "eks_worker_node_policy" {
  # count      = var.resource_type == "kubernetes" && var.action != "delete" ? 1 : 0
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.eks_node_role[0].name
}

resource "aws_iam_role_policy_attachment" "eks_cni_policy" {
  # count      = var.resource_type == "kubernetes" && var.action != "delete" ? 1 : 0
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.eks_node_role[0].name
}

resource "aws_iam_role_policy_attachment" "eks_container_registry_policy" {
  # count      = var.resource_type == "kubernetes" && var.action != "delete" ? 1 : 0
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role       = aws_iam_role.eks_node_role[0].name
}
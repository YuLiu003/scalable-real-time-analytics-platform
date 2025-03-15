variable "region" {
  description = "The AWS region to deploy the resources"
  type        = string
  default     = "us-west-2"
}

variable "cluster_name" {
  description = "The name of the Kubernetes cluster"
  type        = string
  default     = "real-time-analytics-cluster"
}

variable "node_instance_type" {
  description = "The EC2 instance type for the nodes"
  type        = string
  default     = "t3.medium"
}

variable "desired_capacity" {
  description = "The desired number of nodes in the cluster"
  type        = number
  default     = 3
}

variable "max_size" {
  description = "The maximum number of nodes in the cluster"
  type        = number
  default     = 5
}

variable "min_size" {
  description = "The minimum number of nodes in the cluster"
  type        = number
  default     = 1
}

variable "vpc_cidr" {
  description = "The CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "subnet_cidrs" {
  description = "The CIDR blocks for the subnets"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "kafka_bootstrap_servers" {
  description = "The bootstrap servers for Kafka"
  type        = string
  default     = "kafka:9092"
}

variable "influxdb_url" {
  description = "The URL for the InfluxDB instance"
  type        = string
  default     = "http://influxdb:8086"
}
output "kubernetes_cluster_name" {
  value = azurerm_kubernetes_cluster.example.name
}

output "kubernetes_cluster_location" {
  value = azurerm_kubernetes_cluster.example.location
}

output "kubernetes_cluster_resource_group" {
  value = azurerm_resource_group.example.name
}

output "ingestion_service_url" {
  value = "http://${azurerm_kubernetes_cluster.example.kube_admin_config.0.host}:${azurerm_kubernetes_cluster.example.kube_admin_config.0.port}/api"
}

output "processing_service_url" {
  value = "http://${azurerm_kubernetes_cluster.example.kube_admin_config.0.host}:${azurerm_kubernetes_cluster.example.kube_admin_config.0.port}/process"
}

output "storage_service_url" {
  value = "http://${azurerm_kubernetes_cluster.example.kube_admin_config.0.host}:${azurerm_kubernetes_cluster.example.kube_admin_config.0.port}/storage"
}

output "visualization_service_url" {
  value = "http://${azurerm_kubernetes_cluster.example.kube_admin_config.0.host}:${azurerm_kubernetes_cluster.example.kube_admin_config.0.port}/dashboard"
}
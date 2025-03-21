import subprocess
import yaml
import os

class NamespaceManager:
    """Manages Kubernetes namespaces for tenants."""
    
    def create_tenant_namespace(self, tenant_id):
        """Create a namespace for a tenant with proper isolation."""
        namespace = f"tenant-{tenant_id}"
        
        # Create namespace YAML
        namespace_yaml = {
            "apiVersion": "v1",
            "kind": "Namespace",
            "metadata": {
                "name": namespace,
                "labels": {
                    "tenant-id": tenant_id
                }
            }
        }
        
        # Create resource quotas
        quota_yaml = {
            "apiVersion": "v1",
            "kind": "ResourceQuota",
            "metadata": {
                "name": f"{namespace}-quota",
                "namespace": namespace
            },
            "spec": {
                "hard": {
                    "pods": "20",
                    "requests.cpu": "4",
                    "requests.memory": "8Gi",
                    "limits.cpu": "8",
                    "limits.memory": "16Gi"
                }
            }
        }
        
        # Create network policy for isolation
        network_policy_yaml = {
            "apiVersion": "networking.k8s.io/v1",
            "kind": "NetworkPolicy",
            "metadata": {
                "name": f"{namespace}-isolation",
                "namespace": namespace
            },
            "spec": {
                "podSelector": {},
                "ingress": [
                    {
                        "from": [
                            {
                                "namespaceSelector": {
                                    "matchLabels": {
                                        "tenant-id": tenant_id
                                    }
                                }
                            },
                            {
                                "namespaceSelector": {
                                    "matchLabels": {
                                        "name": "analytics-platform-system"
                                    }
                                }
                            }
                        ]
                    }
                ]
            }
        }
        
        # Write to temp files and apply
        os.makedirs("temp", exist_ok=True)
        
        with open("temp/namespace.yaml", "w") as f:
            yaml.dump(namespace_yaml, f)
            
        with open("temp/quota.yaml", "w") as f:
            yaml.dump(quota_yaml, f)
            
        with open("temp/network_policy.yaml", "w") as f:
            yaml.dump(network_policy_yaml, f)
        
        # Apply configurations
        subprocess.run(["kubectl", "apply", "-f", "temp/namespace.yaml"])
        subprocess.run(["kubectl", "apply", "-f", "temp/quota.yaml"])
        subprocess.run(["kubectl", "apply", "-f", "temp/network_policy.yaml"])
        
        print(f"Created tenant namespace: {namespace}")
        return namespace
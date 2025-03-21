import yaml
import os
import subprocess
import time
from kubernetes import client, config
import logging

class K8sTenantManager:
    """Manages Kubernetes resources for tenant isolation."""
    
    def __init__(self, load_kube_config=True):
        """Initialize the K8s tenant manager."""
        self.logger = logging.getLogger(__name__)
        if load_kube_config:
            try:
                # Load kube config for cluster interaction
                config.load_kube_config()
            except Exception as e:
                self.logger.error(f"Failed to load kube config: {e}")
                raise
        
        self.v1 = client.CoreV1Api()
        self.networking_v1 = client.NetworkingV1Api()
        self.apps_v1 = client.AppsV1Api()
        
    def create_tenant(self, tenant_id, plan="basic"):
        """Create all resources needed for a new tenant."""
        self.logger.info(f"Creating tenant: {tenant_id} with plan: {plan}")
        
        # Create namespace
        namespace = f"tenant-{tenant_id}"
        self.create_namespace(namespace, tenant_id)
        
        # Create resource quota based on plan
        self.create_resource_quota(namespace, plan)
        
        # Create network policy for isolation
        self.create_network_policy(namespace, tenant_id)
        
        # Create tenant-specific secrets
        api_key = self.generate_api_key(tenant_id)
        self.create_tenant_secrets(namespace, tenant_id, api_key)
        
        return {
            "tenant_id": tenant_id,
            "namespace": namespace,
            "api_key": api_key,
            "status": "created"
        }
    
    def create_namespace(self, namespace, tenant_id):
        """Create a Kubernetes namespace for a tenant."""
        try:
            # Check if namespace exists
            try:
                self.v1.read_namespace(namespace)
                self.logger.info(f"Namespace {namespace} already exists")
                return
            except client.rest.ApiException as e:
                if e.status != 404:
                    raise
            
            # Create namespace with tenant labels
            body = client.V1Namespace(
                metadata=client.V1ObjectMeta(
                    name=namespace,
                    labels={
                        "tenant-id": tenant_id,
                        "created-by": "tenant-management-system"
                    }
                )
            )
            
            self.v1.create_namespace(body)
            self.logger.info(f"Created namespace: {namespace}")
        except Exception as e:
            self.logger.error(f"Failed to create namespace: {e}")
            raise
    
    def create_resource_quota(self, namespace, plan):
        """Create resource quota for a tenant based on their plan."""
        try:
            # Define quotas based on plan
            quotas = {
                "basic": {
                    "pods": "10",
                    "requests.cpu": "1",
                    "requests.memory": "2Gi",
                    "limits.cpu": "2",
                    "limits.memory": "4Gi"
                },
                "standard": {
                    "pods": "20",
                    "requests.cpu": "4",
                    "requests.memory": "8Gi",
                    "limits.cpu": "8",
                    "limits.memory": "16Gi"
                },
                "premium": {
                    "pods": "50",
                    "requests.cpu": "8",
                    "requests.memory": "16Gi",
                    "limits.cpu": "16",
                    "limits.memory": "32Gi"
                }
            }
            
            # Use the basic plan as default fallback
            plan_quotas = quotas.get(plan, quotas["basic"])
            
            # Create the resource quota
            body = client.V1ResourceQuota(
                metadata=client.V1ObjectMeta(
                    name=f"{namespace}-quota"
                ),
                spec=client.V1ResourceQuotaSpec(
                    hard=plan_quotas
                )
            )
            
            self.v1.create_namespaced_resource_quota(
                namespace=namespace,
                body=body
            )
            self.logger.info(f"Created resource quota for {namespace} with plan {plan}")
        except Exception as e:
            self.logger.error(f"Failed to create resource quota: {e}")
            raise
    
    def create_network_policy(self, namespace, tenant_id):
        """Create network policy for tenant isolation."""
        try:
            # Define network policy
            body = client.V1NetworkPolicy(
                metadata=client.V1ObjectMeta(
                    name=f"{namespace}-isolation"
                ),
                spec=client.V1NetworkPolicySpec(
                    pod_selector=client.V1LabelSelector(),  # Select all pods
                    ingress=[
                        client.V1NetworkPolicyIngressRule(
                            _from=[
                                # Allow from same namespace
                                client.V1NetworkPolicyPeer(
                                    namespace_selector=client.V1LabelSelector(
                                        match_labels={"tenant-id": tenant_id}
                                    )
                                ),
                                # Allow from system namespace
                                client.V1NetworkPolicyPeer(
                                    namespace_selector=client.V1LabelSelector(
                                        match_labels={"name": "analytics-platform-system"}
                                    )
                                )
                            ]
                        )
                    ]
                )
            )
            
            self.networking_v1.create_namespaced_network_policy(
                namespace=namespace,
                body=body
            )
            self.logger.info(f"Created network policy for {namespace}")
        except Exception as e:
            self.logger.error(f"Failed to create network policy: {e}")
            raise
    
    def generate_api_key(self, tenant_id):
        """Generate a unique API key for tenant."""
        import secrets
        import hashlib
        
        # Generate a random string and hash it with tenant_id
        random_part = secrets.token_urlsafe(16)
        hash_input = f"{tenant_id}:{random_part}:{time.time()}"
        hashed = hashlib.sha256(hash_input.encode()).hexdigest()[:20]
        
        # Format key to include tenant prefix for easy identification
        api_key = f"tk_{tenant_id[:6]}_{hashed}"
        return api_key
    
    def create_tenant_secrets(self, namespace, tenant_id, api_key):
        """Create secrets for tenant."""
        try:
            # Create API key secret
            body = client.V1Secret(
                metadata=client.V1ObjectMeta(
                    name="tenant-api-key"
                ),
                string_data={
                    "api-key": api_key
                }
            )
            
            self.v1.create_namespaced_secret(
                namespace=namespace,
                body=body
            )
            self.logger.info(f"Created API key secret for {namespace}")
            
            # Create tenant ID secret
            body = client.V1Secret(
                metadata=client.V1ObjectMeta(
                    name="tenant-id"
                ),
                string_data={
                    "tenant-id": tenant_id
                }
            )
            
            self.v1.create_namespaced_secret(
                namespace=namespace,
                body=body
            )
            self.logger.info(f"Created tenant ID secret for {namespace}")
        except Exception as e:
            self.logger.error(f"Failed to create tenant secrets: {e}")
            raise
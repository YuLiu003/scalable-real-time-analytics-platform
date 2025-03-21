from datetime import datetime

class Tenant:
    """Represents a platform tenant with isolation settings."""
    
    def __init__(self, tenant_id, name, plan="basic"):
        self.tenant_id = tenant_id
        self.name = name
        self.plan = plan  # basic, standard, premium
        self.resource_limits = self._get_plan_limits(plan)
        self.created_at = datetime.now()
        
    def _get_plan_limits(self, plan):
        """Get resource limits based on plan tier."""
        limits = {
            "basic": {
                "max_data_points": 100000,
                "max_queries_per_min": 60,
                "retention_days": 30,
                "max_dashboards": 5,
                "cpu_limit": "2",
                "memory_limit": "2Gi"
            },
            "standard": {
                "max_data_points": 1000000,
                "max_queries_per_min": 300,
                "retention_days": 90,
                "max_dashboards": 20,
                "cpu_limit": "4",
                "memory_limit": "8Gi"
            },
            "premium": {
                "max_data_points": 10000000,
                "max_queries_per_min": 1000,
                "retention_days": 365,
                "max_dashboards": 50,
                "cpu_limit": "8",
                "memory_limit": "16Gi"
            }
        }
        return limits.get(plan, limits["basic"])
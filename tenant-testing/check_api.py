import requests
import json

API_ENDPOINT = "http://localhost:5050"  # Docker Compose tenant-api service

def check_api():
    """Check if the API is running and accessible."""
    try:
        # Check health endpoint
        print("Testing health endpoint...")
        response = requests.get(f"{API_ENDPOINT}/health")
        print(f"Status code: {response.status_code}")
        print(f"Response: {response.text}")
        
        # Try simple GET with no auth
        print("\nTesting GET /api/data without auth...")
        response = requests.get(f"{API_ENDPOINT}/api/data")
        print(f"Status code: {response.status_code}")
        print(f"Response: {response.text}")
        
        # Try GET with auth
        print("\nTesting GET /api/data with auth...")
        response = requests.get(
            f"{API_ENDPOINT}/api/data",
            headers={"X-API-Key": "test-key-1"}
        )
        print(f"Status code: {response.status_code}")
        print(f"Response: {response.text}")
        
        # List available routes if available
        try:
            print("\nListing available routes...")
            response = requests.get(f"{API_ENDPOINT}/routes")
            if response.status_code == 200:
                print(json.dumps(response.json(), indent=2))
            else:
                print(f"Status code: {response.status_code}")
                print(f"Response: {response.text}")
        except requests.RequestException:
            print("Routes endpoint not available")
        
    except requests.RequestException as e:
        print(f"Request failed: {e}")

if __name__ == "__main__":
    check_api()
"""
Test: Coolify proxy endpoints.
"""
from conftest import api_get, api_post


def test_list_apps():
    """GET /api/coolify/apps — list Coolify applications."""
    resp = api_get("/coolify/apps")
    assert resp.status_code == 200, f"List apps failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert isinstance(data, (list, dict)), f"Unexpected type: {type(data)}"
    count = len(data) if isinstance(data, list) else len(data.get("data", data.get("apps", [])))
    print(f"  PASS: Coolify apps listed — {count} entries")


def test_get_app():
    """GET /api/coolify/apps/:uuid — get single app (Bastion itself)."""
    resp = api_get("/coolify/apps/dosgc4go4skko4kc0s4oksg8")
    assert resp.status_code == 200, f"Get app failed: {resp.status_code} {resp.text}"
    print("  PASS: Got Bastion app details from Coolify")


def test_get_app_envs():
    """GET /api/coolify/apps/:uuid/envs — get app env vars."""
    resp = api_get("/coolify/apps/dosgc4go4skko4kc0s4oksg8/envs")
    assert resp.status_code == 200, f"Get envs failed: {resp.status_code} {resp.text}"
    print("  PASS: Got app environment variables")


def test_list_databases():
    """GET /api/coolify/databases — list databases."""
    resp = api_get("/coolify/databases")
    assert resp.status_code == 200, f"List DBs failed: {resp.status_code} {resp.text}"
    print("  PASS: Coolify databases listed")


def test_list_services():
    """GET /api/coolify/services — list services."""
    resp = api_get("/coolify/services")
    assert resp.status_code == 200, f"List services failed: {resp.status_code} {resp.text}"
    print("  PASS: Coolify services listed")


def test_list_deployments():
    """GET /api/coolify/deployments — list deployments."""
    resp = api_get("/coolify/deployments")
    assert resp.status_code == 200, f"List deployments failed: {resp.status_code} {resp.text}"
    print("  PASS: Coolify deployments listed")


if __name__ == "__main__":
    test_list_apps()
    test_get_app()
    test_get_app_envs()
    test_list_databases()
    test_list_services()
    test_list_deployments()
    print("\nALL COOLIFY TESTS PASSED")

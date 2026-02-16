"""
Test: Dashboard and System endpoints.
"""
from conftest import api_get
import requests
from conftest import BASE_URL


def test_dashboard_overview():
    """GET /api/dashboard/overview — aggregated stats."""
    resp = api_get("/dashboard/overview")
    assert resp.status_code == 200, f"Dashboard failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "servers" in data, "Missing servers in dashboard"
    assert "containers" in data or "cron_jobs" in data, "Missing expected dashboard fields"
    print(f"  PASS: Dashboard overview returned — keys: {list(data.keys())}")


def test_dashboard_no_auth():
    """GET /api/dashboard/overview — without auth should 401."""
    resp = requests.get(f"{BASE_URL}/dashboard/overview", timeout=10)
    assert resp.status_code == 401, f"Expected 401, got {resp.status_code}"
    print("  PASS: Dashboard without auth returns 401")


def test_system_info():
    """GET /api/system/info — system information."""
    resp = api_get("/system/info")
    assert resp.status_code == 200, f"System info failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "version" in data, "Missing version"
    print(f"  PASS: System info — version={data.get('version')}")


def test_status_page():
    """GET /api/status — status overview."""
    resp = api_get("/status")
    assert resp.status_code == 200, f"Status failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "status" in data, "Missing status field"
    print(f"  PASS: Status page — status={data.get('status')}")


if __name__ == "__main__":
    test_dashboard_overview()
    test_dashboard_no_auth()
    test_system_info()
    test_status_page()
    print("\nALL DASHBOARD TESTS PASSED")

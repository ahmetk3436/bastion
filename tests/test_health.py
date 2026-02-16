"""
Test: Health endpoint (public, no auth required).
"""
import requests
from conftest import BASE_URL


def test_health():
    """GET /api/health — should return status ok with DB check."""
    resp = requests.get(f"{BASE_URL}/health", timeout=10)
    assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
    data = resp.json()
    assert data["status"] == "ok", f"Health status not ok: {data}"
    assert data["db"] == "ok", f"DB status not ok: {data}"
    assert data["service"] == "bastion", f"Service name mismatch: {data}"
    assert "version" in data, "Missing version field"
    assert "uptime" in data, "Missing uptime field"
    assert "time" in data, "Missing time field"
    print(f"  PASS: Health OK — version={data['version']}, uptime={data['uptime']}")


if __name__ == "__main__":
    test_health()
    print("ALL HEALTH TESTS PASSED")

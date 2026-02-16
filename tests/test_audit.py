"""
Test: Audit log endpoints.
"""
from conftest import api_get


def test_list_audit_logs():
    """GET /api/audit — list audit logs."""
    resp = api_get("/audit")
    assert resp.status_code == 200, f"Audit failed: {resp.status_code} {resp.text}"
    data = resp.json()
    logs = data.get("logs", data)
    assert isinstance(logs, list)
    print(f"  PASS: Listed {len(logs)} audit log entries")


def test_audit_pagination():
    """GET /api/audit?page=1&per_page=5 — paginated."""
    resp = api_get("/audit", params={"page": 1, "per_page": 5})
    assert resp.status_code == 200, f"Paginated audit failed: {resp.status_code}"
    data = resp.json()
    assert "total" in data or "logs" in data, f"Missing fields: {data}"
    print("  PASS: Paginated audit retrieved")


def test_audit_filter_action():
    """GET /api/audit?action=login — filter by action."""
    resp = api_get("/audit", params={"action": "login"})
    assert resp.status_code == 200, f"Filtered audit failed: {resp.status_code}"
    print("  PASS: Filtered audit by action")


if __name__ == "__main__":
    test_list_audit_logs()
    test_audit_pagination()
    test_audit_filter_action()
    print("\nALL AUDIT TESTS PASSED")

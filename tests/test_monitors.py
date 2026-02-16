"""
Test: Monitor (uptime + SSL) endpoints.
"""
from conftest import api_get, api_post, api_delete

MONITOR_ID = None


def test_create_monitor():
    """POST /api/monitors — create uptime monitor."""
    global MONITOR_ID
    resp = api_post("/monitors", json={
        "name": "Test Monitor — Bastion Health",
        "url": "http://89.47.113.196:8097/api/health",
        "type": "http",
        "method": "GET",
        "interval_seconds": 60,
        "timeout_ms": 5000,
        "expected_status": 200,
    })
    assert resp.status_code in [200, 201], f"Create monitor failed: {resp.status_code} {resp.text}"
    data = resp.json()
    monitor = data.get("monitor", data)
    MONITOR_ID = monitor.get("id") or monitor.get("ID")
    print(f"  PASS: Monitor created — id={MONITOR_ID}")


def test_list_monitors():
    """GET /api/monitors — list all monitors."""
    resp = api_get("/monitors")
    assert resp.status_code == 200, f"List monitors failed: {resp.status_code} {resp.text}"
    data = resp.json()
    monitors = data.get("monitors", data)
    assert isinstance(monitors, list)
    print(f"  PASS: Listed {len(monitors)} monitors")


def test_get_monitor():
    """GET /api/monitors/:id — get single monitor with pings."""
    if not MONITOR_ID:
        print("  SKIP: No monitor")
        return
    resp = api_get(f"/monitors/{MONITOR_ID}")
    assert resp.status_code == 200, f"Get monitor failed: {resp.status_code} {resp.text}"
    print("  PASS: Monitor details retrieved")


def test_toggle_monitor():
    """POST /api/monitors/:id/toggle — enable/disable."""
    if not MONITOR_ID:
        print("  SKIP: No monitor")
        return
    resp = api_post(f"/monitors/{MONITOR_ID}/toggle")
    assert resp.status_code == 200, f"Toggle failed: {resp.status_code} {resp.text}"
    print("  PASS: Monitor toggled")


def test_monitor_pings():
    """GET /api/monitors/:id/pings — get ping history."""
    if not MONITOR_ID:
        print("  SKIP: No monitor")
        return
    resp = api_get(f"/monitors/{MONITOR_ID}/pings")
    assert resp.status_code == 200, f"Pings failed: {resp.status_code} {resp.text}"
    print("  PASS: Monitor pings retrieved")


def test_ssl_list():
    """GET /api/monitors/ssl — list SSL certificates."""
    resp = api_get("/monitors/ssl")
    assert resp.status_code == 200, f"SSL list failed: {resp.status_code} {resp.text}"
    print("  PASS: SSL cert list retrieved")


def test_ssl_check():
    """POST /api/monitors/ssl/check — check SSL cert for domain."""
    resp = api_post("/monitors/ssl/check", json={
        "domain": "github.com",
    })
    assert resp.status_code == 200, f"SSL check failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "days_remaining" in data, f"Missing days_remaining: {data}"
    print(f"  PASS: SSL check — github.com days_remaining={data['days_remaining']}")


def test_delete_monitor():
    """DELETE /api/monitors/:id — delete monitor."""
    if not MONITOR_ID:
        print("  SKIP: No monitor")
        return
    resp = api_delete(f"/monitors/{MONITOR_ID}")
    assert resp.status_code == 200, f"Delete failed: {resp.status_code} {resp.text}"
    print("  PASS: Monitor deleted")


if __name__ == "__main__":
    test_create_monitor()
    test_list_monitors()
    test_get_monitor()
    test_toggle_monitor()
    test_monitor_pings()
    test_ssl_list()
    test_ssl_check()
    test_delete_monitor()
    print("\nALL MONITOR TESTS PASSED")

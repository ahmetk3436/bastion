"""
Test: Alert rules and alerts endpoints.
"""
from conftest import api_get, api_post, api_put, api_delete

RULE_ID = None


def test_create_alert_rule():
    """POST /api/alerts/rules — create alert rule."""
    global RULE_ID
    resp = api_post("/alerts/rules", json={
        "name": "Test CPU Alert",
        "type": "metric",
        "metric": "cpu_percent",
        "operator": ">",
        "threshold": 95.0,
        "duration_seconds": 60,
        "notification_channel": "telegram",
    })
    assert resp.status_code in [200, 201], f"Create rule failed: {resp.status_code} {resp.text}"
    data = resp.json()
    rule = data.get("rule", data)
    RULE_ID = rule.get("id") or rule.get("ID")
    print(f"  PASS: Alert rule created — id={RULE_ID}")


def test_list_alert_rules():
    """GET /api/alerts/rules — list all rules."""
    resp = api_get("/alerts/rules")
    assert resp.status_code == 200, f"List rules failed: {resp.status_code} {resp.text}"
    data = resp.json()
    rules = data.get("rules", data)
    assert isinstance(rules, list)
    print(f"  PASS: Listed {len(rules)} alert rules")


def test_list_alerts():
    """GET /api/alerts — list all alerts."""
    resp = api_get("/alerts")
    assert resp.status_code == 200, f"List alerts failed: {resp.status_code} {resp.text}"
    data = resp.json()
    alerts = data.get("alerts", data)
    assert isinstance(alerts, list)
    print(f"  PASS: Listed {len(alerts)} alerts")


def test_list_alerts_filtered():
    """GET /api/alerts?status=firing — filtered alerts."""
    resp = api_get("/alerts", params={"status": "firing"})
    assert resp.status_code == 200, f"Filtered alerts failed: {resp.status_code}"
    print("  PASS: Filtered alerts retrieved")


def test_delete_alert_rule():
    """DELETE /api/alerts/rules/:id — delete rule."""
    if not RULE_ID:
        print("  SKIP: No rule")
        return
    resp = api_delete(f"/alerts/rules/{RULE_ID}")
    assert resp.status_code == 200, f"Delete rule failed: {resp.status_code} {resp.text}"
    print("  PASS: Alert rule deleted")


if __name__ == "__main__":
    test_create_alert_rule()
    test_list_alert_rules()
    test_list_alerts()
    test_list_alerts_filtered()
    test_delete_alert_rule()
    print("\nALL ALERT TESTS PASSED")

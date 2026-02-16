"""
Test: Cron job CRUD and execution endpoints.
"""
from conftest import api_get, api_post, api_put, api_delete, SSH_HOST, SSH_USER, SSH_PASS

SERVER_ID = None
CRON_ID = None


def setup_server():
    global SERVER_ID
    resp = api_post("/servers", json={
        "name": "Cron Test Server",
        "host": SSH_HOST, "port": 22,
        "username": SSH_USER, "password": SSH_PASS,
        "auth_type": "password",
    })
    data = resp.json()
    server = data.get("server", data)
    SERVER_ID = server.get("id") or server.get("ID")
    print(f"  Setup: Server id={SERVER_ID}")


def test_create_cron():
    """POST /api/servers/:id/crons — create cron job."""
    global CRON_ID
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_post(f"/servers/{SERVER_ID}/crons", json={
        "name": "Test Cron Job",
        "schedule": "*/5 * * * *",
        "command": "echo 'cron test'",
        "notification_on_failure": False,
    })
    assert resp.status_code in [200, 201], f"Create cron failed: {resp.status_code} {resp.text}"
    data = resp.json()
    cron = data.get("cron", data)
    CRON_ID = cron.get("id") or cron.get("ID")
    print(f"  PASS: Cron created — id={CRON_ID}")


def test_list_crons():
    """GET /api/servers/:id/crons — list cron jobs."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_get(f"/servers/{SERVER_ID}/crons")
    assert resp.status_code == 200, f"List crons failed: {resp.status_code} {resp.text}"
    data = resp.json()
    crons = data.get("crons", data)
    assert isinstance(crons, list)
    print(f"  PASS: Listed {len(crons)} cron jobs")


def test_update_cron():
    """PUT /api/crons/:id — update cron job."""
    if not CRON_ID:
        print("  SKIP: No cron")
        return
    resp = api_put(f"/crons/{CRON_ID}", json={
        "name": "Updated Cron Job",
        "schedule": "0 * * * *",
    })
    assert resp.status_code == 200, f"Update cron failed: {resp.status_code} {resp.text}"
    print("  PASS: Cron updated")


def test_toggle_cron():
    """POST /api/crons/:id/toggle — enable/disable cron."""
    if not CRON_ID:
        print("  SKIP: No cron")
        return
    resp = api_post(f"/crons/{CRON_ID}/toggle")
    assert resp.status_code == 200, f"Toggle failed: {resp.status_code} {resp.text}"
    data = resp.json()
    print(f"  PASS: Cron toggled — enabled={data.get('enabled')}")


def test_run_cron():
    """POST /api/crons/:id/run — manually trigger cron."""
    if not CRON_ID:
        print("  SKIP: No cron")
        return
    resp = api_post(f"/crons/{CRON_ID}/run")
    assert resp.status_code == 200, f"Run cron failed: {resp.status_code} {resp.text}"
    data = resp.json()
    print(f"  PASS: Cron executed — output={data.get('output', '').strip()}")


def test_cron_logs():
    """GET /api/crons/:id/logs — get cron logs."""
    if not CRON_ID:
        print("  SKIP: No cron")
        return
    resp = api_get(f"/crons/{CRON_ID}/logs")
    assert resp.status_code == 200, f"Cron logs failed: {resp.status_code} {resp.text}"
    print("  PASS: Cron logs retrieved")


def test_delete_cron():
    """DELETE /api/crons/:id — delete cron job."""
    if not CRON_ID:
        print("  SKIP: No cron")
        return
    resp = api_delete(f"/crons/{CRON_ID}")
    assert resp.status_code == 200, f"Delete cron failed: {resp.status_code} {resp.text}"
    print("  PASS: Cron deleted")


def cleanup():
    if SERVER_ID:
        api_delete(f"/servers/{SERVER_ID}")
        print("  Cleanup: Server deleted")


if __name__ == "__main__":
    setup_server()
    test_create_cron()
    test_list_crons()
    test_update_cron()
    test_toggle_cron()
    test_run_cron()
    test_cron_logs()
    test_delete_cron()
    cleanup()
    print("\nALL CRON TESTS PASSED")

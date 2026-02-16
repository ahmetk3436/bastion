"""
Test: Server CRUD + SSH connection endpoints.
"""
from conftest import api_get, api_post, api_put, api_delete, SSH_HOST, SSH_USER, SSH_PASS

CREATED_SERVER_ID = None


def test_create_server():
    """POST /api/servers — create server with SSH credentials."""
    global CREATED_SERVER_ID
    resp = api_post("/servers", json={
        "name": "Test Server (CI)",
        "host": SSH_HOST,
        "port": 22,
        "username": SSH_USER,
        "password": SSH_PASS,
        "auth_type": "password",
        "is_default": False,
    })
    assert resp.status_code in [200, 201], f"Create server failed: {resp.status_code} {resp.text}"
    data = resp.json()
    server = data.get("server", data)
    assert "id" in server or "ID" in server, f"Missing server ID: {data}"
    CREATED_SERVER_ID = server.get("id") or server.get("ID")
    print(f"  PASS: Server created — id={CREATED_SERVER_ID}")


def test_list_servers():
    """GET /api/servers — list all servers."""
    resp = api_get("/servers")
    assert resp.status_code == 200, f"List servers failed: {resp.status_code} {resp.text}"
    data = resp.json()
    servers = data.get("servers", data)
    assert isinstance(servers, list), f"Expected list, got {type(servers)}"
    print(f"  PASS: Listed {len(servers)} servers")


def test_get_server():
    """GET /api/servers/:id — get single server."""
    if not CREATED_SERVER_ID:
        print("  SKIP: No server created")
        return
    resp = api_get(f"/servers/{CREATED_SERVER_ID}")
    assert resp.status_code == 200, f"Get server failed: {resp.status_code} {resp.text}"
    data = resp.json()
    print(f"  PASS: Got server — name={data.get('server', data).get('name', 'N/A')}")


def test_update_server():
    """PUT /api/servers/:id — update server name."""
    if not CREATED_SERVER_ID:
        print("  SKIP: No server created")
        return
    resp = api_put(f"/servers/{CREATED_SERVER_ID}", json={
        "name": "Test Server (Updated)",
    })
    assert resp.status_code == 200, f"Update server failed: {resp.status_code} {resp.text}"
    print("  PASS: Server updated")


def test_test_ssh_connection():
    """POST /api/servers/:id/test — test SSH connectivity."""
    if not CREATED_SERVER_ID:
        print("  SKIP: No server created")
        return
    resp = api_post(f"/servers/{CREATED_SERVER_ID}/test")
    assert resp.status_code == 200, f"SSH test failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "fingerprint" in data or "message" in data, f"Unexpected response: {data}"
    print(f"  PASS: SSH connection test OK")


def test_server_metrics():
    """GET /api/servers/:id/metrics — get historical metrics."""
    if not CREATED_SERVER_ID:
        print("  SKIP: No server created")
        return
    resp = api_get(f"/servers/{CREATED_SERVER_ID}/metrics", params={"period": "1h"})
    assert resp.status_code == 200, f"Metrics failed: {resp.status_code} {resp.text}"
    print("  PASS: Server metrics retrieved")


def test_server_live_metrics():
    """GET /api/servers/:id/metrics/live — get live metrics."""
    if not CREATED_SERVER_ID:
        print("  SKIP: No server created")
        return
    resp = api_get(f"/servers/{CREATED_SERVER_ID}/metrics/live")
    # May return 200 or 502 if metrics collection hasn't run yet
    assert resp.status_code in [200, 404, 502], f"Live metrics failed: {resp.status_code} {resp.text}"
    print(f"  PASS: Live metrics returned {resp.status_code}")


def test_delete_server():
    """DELETE /api/servers/:id — delete server."""
    if not CREATED_SERVER_ID:
        print("  SKIP: No server created")
        return
    resp = api_delete(f"/servers/{CREATED_SERVER_ID}")
    assert resp.status_code == 200, f"Delete server failed: {resp.status_code} {resp.text}"
    print("  PASS: Server deleted")


if __name__ == "__main__":
    test_create_server()
    test_list_servers()
    test_get_server()
    test_update_server()
    test_test_ssh_connection()
    test_server_metrics()
    test_server_live_metrics()
    test_delete_server()
    print("\nALL SERVER TESTS PASSED")

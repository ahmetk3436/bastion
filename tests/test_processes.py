"""
Test: Process and service management endpoints.
"""
from conftest import api_get, api_post, api_delete, SSH_HOST, SSH_USER, SSH_PASS

SERVER_ID = None


def setup_server():
    global SERVER_ID
    resp = api_post("/servers", json={
        "name": "Process Test Server",
        "host": SSH_HOST, "port": 22,
        "username": SSH_USER, "password": SSH_PASS,
        "auth_type": "password",
    })
    data = resp.json()
    server = data.get("server", data)
    SERVER_ID = server.get("id") or server.get("ID")
    print(f"  Setup: Server id={SERVER_ID}")


def test_list_processes():
    """GET /api/servers/:id/processes — list top processes."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_get(f"/servers/{SERVER_ID}/processes")
    assert resp.status_code == 200, f"Processes failed: {resp.status_code} {resp.text}"
    data = resp.json()
    procs = data.get("processes", data)
    assert isinstance(procs, list)
    print(f"  PASS: Listed {len(procs)} processes")


def test_list_services():
    """GET /api/servers/:id/services — list systemd services."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_get(f"/servers/{SERVER_ID}/services")
    assert resp.status_code == 200, f"Services failed: {resp.status_code} {resp.text}"
    data = resp.json()
    services = data.get("services", data)
    assert isinstance(services, list)
    print(f"  PASS: Listed {len(services)} services")


def test_network_connections():
    """GET /api/servers/:id/network/connections — active connections."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_get(f"/servers/{SERVER_ID}/network/connections")
    assert resp.status_code == 200, f"Network failed: {resp.status_code} {resp.text}"
    data = resp.json()
    conns = data.get("connections", data)
    assert isinstance(conns, list)
    print(f"  PASS: Listed {len(conns)} network connections")


def cleanup():
    if SERVER_ID:
        api_delete(f"/servers/{SERVER_ID}")
        print("  Cleanup: Server deleted")


if __name__ == "__main__":
    setup_server()
    test_list_processes()
    test_list_services()
    test_network_connections()
    cleanup()
    print("\nALL PROCESS TESTS PASSED")

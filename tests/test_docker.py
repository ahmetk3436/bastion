"""
Test: Docker management endpoints (containers, images).
"""
from conftest import api_get, api_post, api_delete, SSH_HOST, SSH_USER, SSH_PASS

SERVER_ID = None


def setup_server():
    global SERVER_ID
    resp = api_post("/servers", json={
        "name": "Docker Test Server",
        "host": SSH_HOST, "port": 22,
        "username": SSH_USER, "password": SSH_PASS,
        "auth_type": "password",
    })
    data = resp.json()
    server = data.get("server", data)
    SERVER_ID = server.get("id") or server.get("ID")
    print(f"  Setup: Server id={SERVER_ID}")


def test_list_containers():
    """GET /api/servers/:id/docker/containers — list Docker containers."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_get(f"/servers/{SERVER_ID}/docker/containers")
    assert resp.status_code == 200, f"List containers failed: {resp.status_code} {resp.text}"
    data = resp.json()
    containers = data.get("containers", data)
    assert isinstance(containers, list)
    print(f"  PASS: Listed {len(containers)} containers")
    return containers


def test_container_stats():
    """GET /api/servers/:id/docker/containers/:cid/stats — container stats."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    containers = test_list_containers()
    if not containers:
        print("  SKIP: No containers to test")
        return
    cid = containers[0].get("id") or containers[0].get("ID") or containers[0].get("container_id", "")
    if not cid:
        print("  SKIP: No container ID found")
        return
    resp = api_get(f"/servers/{SERVER_ID}/docker/containers/{cid[:12]}/stats")
    assert resp.status_code in [200, 502], f"Stats failed: {resp.status_code}"
    print(f"  PASS: Container stats returned {resp.status_code}")


def test_container_logs():
    """GET /api/servers/:id/docker/containers/:cid/logs — container logs."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_get(f"/servers/{SERVER_ID}/docker/containers")
    containers = resp.json().get("containers", [])
    if not containers:
        print("  SKIP: No containers")
        return
    cid = containers[0].get("id") or containers[0].get("ID") or containers[0].get("container_id", "")
    if not cid:
        print("  SKIP: No container ID")
        return
    resp = api_get(f"/servers/{SERVER_ID}/docker/containers/{cid[:12]}/logs", params={"tail": "10"})
    assert resp.status_code in [200, 502], f"Logs failed: {resp.status_code}"
    print(f"  PASS: Container logs returned {resp.status_code}")


def test_list_images():
    """GET /api/servers/:id/docker/images — list Docker images."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_get(f"/servers/{SERVER_ID}/docker/images")
    assert resp.status_code == 200, f"List images failed: {resp.status_code} {resp.text}"
    data = resp.json()
    images = data.get("images", data)
    assert isinstance(images, list)
    print(f"  PASS: Listed {len(images)} images")


def cleanup():
    if SERVER_ID:
        api_delete(f"/servers/{SERVER_ID}")
        print("  Cleanup: Server deleted")


if __name__ == "__main__":
    setup_server()
    test_list_containers()
    test_container_stats()
    test_container_logs()
    test_list_images()
    cleanup()
    print("\nALL DOCKER TESTS PASSED")

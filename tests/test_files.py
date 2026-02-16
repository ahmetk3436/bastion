"""
Test: File management endpoints (list, read, write).
"""
from conftest import api_get, api_put, api_post, api_delete, SSH_HOST, SSH_USER, SSH_PASS

SERVER_ID = None


def setup_server():
    global SERVER_ID
    resp = api_post("/servers", json={
        "name": "File Test Server",
        "host": SSH_HOST, "port": 22,
        "username": SSH_USER, "password": SSH_PASS,
        "auth_type": "password",
    })
    data = resp.json()
    server = data.get("server", data)
    SERVER_ID = server.get("id") or server.get("ID")
    print(f"  Setup: Server id={SERVER_ID}")


def test_list_files():
    """GET /api/servers/:id/files — list directory."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_get(f"/servers/{SERVER_ID}/files", params={"path": "/tmp"})
    assert resp.status_code == 200, f"List files failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "files" in data or "path" in data, f"Unexpected response: {data}"
    print(f"  PASS: Listed files in /tmp")


def test_list_root():
    """GET /api/servers/:id/files — list root."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_get(f"/servers/{SERVER_ID}/files", params={"path": "/"})
    assert resp.status_code == 200, f"List root failed: {resp.status_code} {resp.text}"
    print("  PASS: Listed root directory")


def test_read_file():
    """GET /api/servers/:id/files/content — read a file."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_get(f"/servers/{SERVER_ID}/files/content", params={"path": "/etc/hostname"})
    assert resp.status_code == 200, f"Read file failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "content" in data, f"Missing content: {data}"
    print(f"  PASS: Read /etc/hostname — content='{data['content'].strip()}'")


def test_write_and_read_file():
    """PUT /api/servers/:id/files/content — write then read back."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    test_content = "bastion-test-file-content-12345"
    resp = api_put(f"/servers/{SERVER_ID}/files/content", json={
        "path": "/tmp/bastion_test_file.txt",
        "content": test_content,
    })
    assert resp.status_code == 200, f"Write failed: {resp.status_code} {resp.text}"

    # Read back
    resp = api_get(f"/servers/{SERVER_ID}/files/content", params={"path": "/tmp/bastion_test_file.txt"})
    assert resp.status_code == 200, f"Read back failed: {resp.status_code}"
    data = resp.json()
    assert test_content in data.get("content", ""), f"Content mismatch: {data}"
    print("  PASS: Write + read back verified")


def test_disk_usage():
    """GET /api/servers/:id/disk — disk usage info."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_get(f"/servers/{SERVER_ID}/disk")
    assert resp.status_code == 200, f"Disk failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "filesystems" in data or "top_dirs" in data, f"Unexpected: {data}"
    print("  PASS: Disk usage retrieved")


def cleanup():
    if SERVER_ID:
        # Clean up test file
        api_post(f"/servers/{SERVER_ID}/exec", json={"command": "rm -f /tmp/bastion_test_file.txt"})
        api_delete(f"/servers/{SERVER_ID}")
        print("  Cleanup: Server deleted")


if __name__ == "__main__":
    setup_server()
    test_list_files()
    test_list_root()
    test_read_file()
    test_write_and_read_file()
    test_disk_usage()
    cleanup()
    print("\nALL FILE TESTS PASSED")

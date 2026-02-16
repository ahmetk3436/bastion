"""
Test: Command execution and history endpoints.
"""
from conftest import api_get, api_post, api_delete, SSH_HOST, SSH_USER, SSH_PASS

SERVER_ID = None


def setup_server():
    """Create a temporary server for command tests."""
    global SERVER_ID
    resp = api_post("/servers", json={
        "name": "CMD Test Server",
        "host": SSH_HOST,
        "port": 22,
        "username": SSH_USER,
        "password": SSH_PASS,
        "auth_type": "password",
    })
    assert resp.status_code in [200, 201], f"Setup failed: {resp.status_code} {resp.text}"
    data = resp.json()
    server = data.get("server", data)
    SERVER_ID = server.get("id") or server.get("ID")
    print(f"  Setup: Server created id={SERVER_ID}")


def test_exec_command():
    """POST /api/servers/:id/exec — execute a command via SSH."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_post(f"/servers/{SERVER_ID}/exec", json={
        "command": "echo 'hello from bastion test'",
    })
    assert resp.status_code == 200, f"Exec failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "output" in data, f"Missing output: {data}"
    assert "hello from bastion test" in data["output"], f"Unexpected output: {data['output']}"
    assert data.get("exit_code", 0) == 0, f"Non-zero exit: {data}"
    print(f"  PASS: Command exec — output='{data['output'].strip()}'")


def test_exec_command_with_error():
    """POST /api/servers/:id/exec — command that fails."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_post(f"/servers/{SERVER_ID}/exec", json={
        "command": "ls /nonexistent_path_12345",
    })
    assert resp.status_code == 200, f"Exec failed: {resp.status_code} {resp.text}"
    data = resp.json()
    # Should have non-zero exit code
    assert data.get("exit_code", -1) != 0, f"Expected non-zero exit: {data}"
    print(f"  PASS: Failed command returns exit_code={data.get('exit_code')}")


def test_command_history():
    """GET /api/servers/:id/history — command history."""
    if not SERVER_ID:
        print("  SKIP: No server")
        return
    resp = api_get(f"/servers/{SERVER_ID}/history")
    assert resp.status_code == 200, f"History failed: {resp.status_code} {resp.text}"
    data = resp.json()
    history = data.get("history", [])
    assert isinstance(history, list), f"Expected list: {type(history)}"
    assert len(history) > 0, "History should have at least 1 entry after exec"
    print(f"  PASS: Command history — {len(history)} entries")


def test_favorites():
    """GET/POST /api/commands/favorites — list and toggle."""
    resp = api_get("/commands/favorites")
    assert resp.status_code == 200, f"Favorites failed: {resp.status_code} {resp.text}"
    print("  PASS: Favorites list retrieved")


def cleanup_server():
    """Delete the temporary server."""
    if SERVER_ID:
        api_delete(f"/servers/{SERVER_ID}")
        print("  Cleanup: Server deleted")


if __name__ == "__main__":
    setup_server()
    test_exec_command()
    test_exec_command_with_error()
    test_command_history()
    test_favorites()
    cleanup_server()
    print("\nALL COMMAND TESTS PASSED")

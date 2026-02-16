"""
Test: Authentication endpoints — login, refresh, me, password change.
"""
import requests
from conftest import BASE_URL, ADMIN_USERNAME, ADMIN_PASSWORD, get_tokens, auth_headers, api_get, api_put


def test_login_success():
    """POST /api/auth/login — valid credentials."""
    resp = requests.post(f"{BASE_URL}/auth/login", json={
        "username": ADMIN_USERNAME,
        "password": ADMIN_PASSWORD,
    }, timeout=10)
    assert resp.status_code == 200, f"Login failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "access_token" in data, "Missing access_token"
    assert "refresh_token" in data, "Missing refresh_token"
    assert data["user"]["username"] == ADMIN_USERNAME
    assert data["user"]["role"] == "admin"
    assert "display_name" in data["user"]
    assert "avatar_initials" in data["user"]
    print(f"  PASS: Login success — user={data['user']['username']}, role={data['user']['role']}")


def test_login_wrong_password():
    """POST /api/auth/login — wrong password should return 401."""
    resp = requests.post(f"{BASE_URL}/auth/login", json={
        "username": ADMIN_USERNAME,
        "password": "wrong_password_123",
    }, timeout=10)
    assert resp.status_code == 401, f"Expected 401, got {resp.status_code}"
    data = resp.json()
    assert data["error"] is True
    assert data["message"] == "Invalid credentials"
    print("  PASS: Wrong password correctly returns 401")


def test_login_wrong_username():
    """POST /api/auth/login — wrong username should return 401."""
    resp = requests.post(f"{BASE_URL}/auth/login", json={
        "username": "nonexistent_user",
        "password": ADMIN_PASSWORD,
    }, timeout=10)
    assert resp.status_code == 401, f"Expected 401, got {resp.status_code}"
    data = resp.json()
    assert data["error"] is True
    assert data["message"] == "Invalid credentials"
    print("  PASS: Wrong username correctly returns 401")


def test_login_empty_body():
    """POST /api/auth/login — empty body should return 400 or 401."""
    resp = requests.post(f"{BASE_URL}/auth/login", json={}, timeout=10)
    assert resp.status_code in [400, 401], f"Expected 400/401, got {resp.status_code}"
    print(f"  PASS: Empty body returns {resp.status_code}")


def test_login_no_content_type():
    """POST /api/auth/login — no JSON content type."""
    resp = requests.post(f"{BASE_URL}/auth/login", data="not json", timeout=10)
    assert resp.status_code in [400, 401, 415, 422], f"Expected error, got {resp.status_code}"
    print(f"  PASS: No content type returns {resp.status_code}")


def test_refresh_token():
    """POST /api/auth/refresh — refresh access token."""
    _, refresh = get_tokens()
    resp = requests.post(f"{BASE_URL}/auth/refresh", json={
        "refresh_token": refresh,
    }, timeout=10)
    assert resp.status_code == 200, f"Refresh failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "access_token" in data, "Missing new access_token"
    assert "refresh_token" in data, "Missing new refresh_token"
    print("  PASS: Token refresh works")


def test_refresh_invalid_token():
    """POST /api/auth/refresh — invalid token should fail."""
    resp = requests.post(f"{BASE_URL}/auth/refresh", json={
        "refresh_token": "invalid.token.here",
    }, timeout=10)
    assert resp.status_code in [400, 401], f"Expected error, got {resp.status_code}"
    print(f"  PASS: Invalid refresh token returns {resp.status_code}")


def test_me_endpoint():
    """GET /api/auth/me — get current user info."""
    resp = api_get("/auth/me")
    assert resp.status_code == 200, f"Me failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert data["username"] == ADMIN_USERNAME
    assert data["role"] == "admin"
    print(f"  PASS: /auth/me returns user={data['username']}")


def test_me_no_auth():
    """GET /api/auth/me — without token should return 401."""
    resp = requests.get(f"{BASE_URL}/auth/me", timeout=10)
    assert resp.status_code == 401, f"Expected 401, got {resp.status_code}"
    print("  PASS: /auth/me without auth returns 401")


def test_password_change_wrong_old():
    """PUT /api/auth/password — wrong old password."""
    resp = api_put("/auth/password", json={
        "old_password": "definitely_wrong",
        "new_password": "NewPassword123!",
    })
    assert resp.status_code in [400, 401], f"Expected error, got {resp.status_code}"
    print(f"  PASS: Wrong old password returns {resp.status_code}")


def test_password_change_too_short():
    """PUT /api/auth/password — new password too short."""
    resp = api_put("/auth/password", json={
        "old_password": ADMIN_PASSWORD,
        "new_password": "short",
    })
    assert resp.status_code == 400, f"Expected 400, got {resp.status_code}"
    print("  PASS: Short new password returns 400")


if __name__ == "__main__":
    test_login_success()
    test_login_wrong_password()
    test_login_wrong_username()
    test_login_empty_body()
    test_login_no_content_type()
    test_refresh_token()
    test_refresh_invalid_token()
    test_me_endpoint()
    test_me_no_auth()
    test_password_change_wrong_old()
    test_password_change_too_short()
    print("\nALL AUTH TESTS PASSED")

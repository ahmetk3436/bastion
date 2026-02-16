"""
Bastion Backend — Shared test configuration and fixtures.
"""
import os
import requests

BASE_URL = os.getenv("BASTION_URL", "http://89.47.113.196:8097/api")
ADMIN_USERNAME = os.getenv("BASTION_USER", "ahmet")
ADMIN_PASSWORD = os.getenv("BASTION_PASS", "BastionSecure2026")

# SSH credentials for server tests
SSH_HOST = "89.47.113.196"
SSH_USER = "root"
SSH_PASS = "~gM8@Ha4ZXTAJ{V0"


def get_tokens():
    """Login and return (access_token, refresh_token)."""
    resp = requests.post(f"{BASE_URL}/auth/login", json={
        "username": ADMIN_USERNAME,
        "password": ADMIN_PASSWORD,
    }, timeout=10)
    resp.raise_for_status()
    data = resp.json()
    return data["access_token"], data["refresh_token"]


def auth_headers():
    """Return Authorization header dict."""
    token, _ = get_tokens()
    return {"Authorization": f"Bearer {token}"}


def api_get(path, params=None):
    return requests.get(f"{BASE_URL}{path}", headers=auth_headers(), params=params, timeout=15)


def api_post(path, json=None):
    return requests.post(f"{BASE_URL}{path}", headers=auth_headers(), json=json, timeout=30)


def api_put(path, json=None):
    return requests.put(f"{BASE_URL}{path}", headers=auth_headers(), json=json, timeout=15)


def api_delete(path):
    return requests.delete(f"{BASE_URL}{path}", headers=auth_headers(), timeout=15)

"""
Test: Ops integration proxy endpoints (SRE, tickets, reviews).
"""
from conftest import api_get


def test_ops_overview():
    """GET /api/ops/overview — aggregated ops data."""
    resp = api_get("/ops/overview")
    assert resp.status_code == 200, f"Ops overview failed: {resp.status_code} {resp.text}"
    data = resp.json()
    print(f"  PASS: Ops overview — keys: {list(data.keys())}")


def test_sre_events():
    """GET /api/ops/sre/events — SRE events."""
    resp = api_get("/ops/sre/events")
    assert resp.status_code == 200, f"SRE events failed: {resp.status_code} {resp.text}"
    print("  PASS: SRE events retrieved")


def test_support_tickets():
    """GET /api/ops/tickets — support tickets."""
    resp = api_get("/ops/tickets")
    assert resp.status_code == 200, f"Tickets failed: {resp.status_code} {resp.text}"
    print("  PASS: Support tickets retrieved")


def test_reviews():
    """GET /api/ops/reviews — reputation reviews."""
    resp = api_get("/ops/reviews")
    assert resp.status_code == 200, f"Reviews failed: {resp.status_code} {resp.text}"
    print("  PASS: Reviews retrieved")


if __name__ == "__main__":
    test_ops_overview()
    test_sre_events()
    test_support_tickets()
    test_reviews()
    print("\nALL OPS TESTS PASSED")

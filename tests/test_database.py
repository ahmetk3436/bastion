"""
Test: Database management endpoints.
"""
from conftest import api_get, api_post


def test_list_tables():
    """GET /api/database/tables — list all tables."""
    resp = api_get("/database/tables")
    assert resp.status_code == 200, f"List tables failed: {resp.status_code} {resp.text}"
    data = resp.json()
    tables = data.get("tables", data)
    assert isinstance(tables, list)
    assert len(tables) > 0, "Expected at least 1 table"
    print(f"  PASS: Listed {len(tables)} tables")
    return tables


def test_get_table_rows():
    """GET /api/database/tables/:name/rows — get rows from table."""
    tables = test_list_tables()
    if not tables:
        print("  SKIP: No tables")
        return
    table_name = tables[0].get("name", tables[0]) if isinstance(tables[0], dict) else tables[0]
    resp = api_get(f"/database/tables/{table_name}/rows", params={"limit": 5})
    assert resp.status_code == 200, f"Get rows failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "rows" in data or "columns" in data, f"Unexpected response: {list(data.keys())}"
    print(f"  PASS: Got rows from table '{table_name}'")


def test_read_only_query():
    """POST /api/database/query — execute read-only SQL."""
    resp = api_post("/database/query", json={
        "query": "SELECT COUNT(*) as cnt FROM servers",
    })
    assert resp.status_code == 200, f"Query failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "rows" in data, f"Missing rows: {data}"
    print(f"  PASS: Read-only query — result: {data.get('rows')}")


def test_mutation_blocked():
    """POST /api/database/query — mutation should be blocked."""
    resp = api_post("/database/query", json={
        "query": "DELETE FROM servers WHERE id = 'fake-id'",
    })
    assert resp.status_code in [400, 403], f"Expected block, got {resp.status_code}"
    print(f"  PASS: Mutation blocked with {resp.status_code}")


def test_drop_blocked():
    """POST /api/database/query — DROP should be blocked."""
    resp = api_post("/database/query", json={
        "query": "DROP TABLE servers",
    })
    assert resp.status_code in [400, 403], f"Expected block, got {resp.status_code}"
    print(f"  PASS: DROP blocked with {resp.status_code}")


def test_database_stats():
    """GET /api/database/stats — database statistics."""
    resp = api_get("/database/stats")
    assert resp.status_code == 200, f"Stats failed: {resp.status_code} {resp.text}"
    data = resp.json()
    assert "database_size" in data or "version" in data, f"Unexpected response: {data}"
    print(f"  PASS: DB stats — size={data.get('database_size', 'N/A')}")


if __name__ == "__main__":
    test_list_tables()
    test_get_table_rows()
    test_read_only_query()
    test_mutation_blocked()
    test_drop_blocked()
    test_database_stats()
    print("\nALL DATABASE TESTS PASSED")

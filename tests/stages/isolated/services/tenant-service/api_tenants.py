"""Tenant: /v1/tenants/* (companies, email-domains, departments, teams, profile, validate).

Spec: specs/backend-go/tdd/services/tenant-service.md.
"""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from check_framework import Context, run_single
from orca_api_session import find_value, json_body

SUITE = "tenants"
ZERO_UUID = "00000000-0000-0000-0000-000000000000"


def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    me = admin.user or {}
    user_id = me.get("id", "")
    tenant_id = find_value(me, "tenantId", "tenant_id") or ZERO_UUID

    ctx.expect("GET /v1/tenants/validate", admin.get("/v1/tenants/validate", params={"tenant_id": tenant_id}),
               "handled")
    ctx.expect("GET /v1/tenants/validate thiếu tenant_id -> 400",
               admin.get("/v1/tenants/validate"), 400, "handled")
    ctx.expect("GET /v1/tenants/profile", admin.get("/v1/tenants/profile", params={"user_id": user_id}),
               "handled")
    ctx.expect("GET /v1/tenants/profile thiếu user_id -> 400", admin.get("/v1/tenants/profile"), 400, "handled")

    if not ctx.need_writes("tenant writes"):
        return

    # --- company + email domains ---
    r = admin.post("/v1/tenants/companies", json={"name": ctx.name("company")})
    ctx.expect("POST /v1/tenants/companies", r, "2xx", 403)  # tạo công ty có thể giới hạn cho super-admin
    company_id = find_value(json_body(r), "id", "company_id", "companyId")
    ctx.expect("POST /v1/tenants/companies thiếu name -> 400", admin.post("/v1/tenants/companies", json={}),
               400, 403)
    if not company_id:
        ctx.skip("company/domains/department/team", "không tạo được company")
        return
    domain = f"{ctx.name('d')}.example.test"
    ctx.expect("POST /v1/tenants/companies/{id}/email-domains", admin.post(
               f"/v1/tenants/companies/{company_id}/email-domains",
               template="/v1/tenants/companies/{id}/email-domains", json={"email_domain": domain}), "2xx")
    r = admin.get(f"/v1/tenants/companies/{company_id}/email-domains",
                  template="/v1/tenants/companies/{id}/email-domains")
    ctx.expect("GET /v1/tenants/companies/{id}/email-domains", r, 200)
    ctx.assert_true("email-domain vừa thêm có trong danh sách", domain in r.text, r.text[:200])
    ctx.expect("DELETE /v1/tenants/email-domains/{domain}", admin.delete(
               f"/v1/tenants/email-domains/{domain}", template="/v1/tenants/email-domains/{domain}"), "2xx")

    # --- department ---
    r = admin.post("/v1/tenants/departments", json={"company_id": company_id, "name": ctx.name("dept")})
    ctx.expect("POST /v1/tenants/departments", r, "2xx")
    dept_id = find_value(json_body(r), "id", "department_id", "departmentId")
    if dept_id and user_id:
        ctx.expect("PUT /v1/tenants/users/{id}/department", admin.put(
                   f"/v1/tenants/users/{user_id}/department", template="/v1/tenants/users/{id}/department",
                   json={"department_id": dept_id}), "handled")
    else:
        ctx.skip("PUT /v1/tenants/users/{id}/department", "thiếu department/user id")

    # --- team + members ---
    r = admin.post("/v1/tenants/teams", json={"company_id": company_id, "name": ctx.name("team"),
                                              "settings_json": "{}"})
    ctx.expect("POST /v1/tenants/teams", r, "2xx")
    team_id = find_value(json_body(r), "id", "team_id", "teamId")
    if team_id and user_id:
        ctx.expect("POST /v1/tenants/teams/{id}/members", admin.post(
                   f"/v1/tenants/teams/{team_id}/members", template="/v1/tenants/teams/{id}/members",
                   json={"user_id": user_id, "priority": 1}), "2xx")
        r = admin.get(f"/v1/tenants/teams/{team_id}/members", template="/v1/tenants/teams/{id}/members")
        ctx.expect("GET /v1/tenants/teams/{id}/members", r, 200)
    else:
        ctx.skip("team members", "thiếu team/user id")
    ctx.expect("POST /v1/tenants/teams/{id}/members team không tồn tại",
               admin.post(f"/v1/tenants/teams/{ZERO_UUID}/members", template="/v1/tenants/teams/{id}/members",
                          json={"user_id": user_id or ZERO_UUID, "priority": 1}), "4xx")


if __name__ == "__main__":
    run_single(SUITE, run)

"""MOD-Gateway: MCP JSON-RPC (HTTP, stdio)."""
import json
import os

SCHEMA_FILE = os.path.join(os.path.dirname(os.path.dirname(__file__)), "schemas", "mcp-tools.schema.json")


def tool_catalog():
    """Tool catalogue (name, description, input schema, write/review flags) written to schemas/mcp-tools.schema.json."""
    from .tools import TOOLS
    return {"$comment": "GENERATED from tsagent/gateway/tools.py (tsagent ontology codegen); do not edit by hand",
            "tools": [{"name": t["name"], "description": t["description"], "write": t["write"], "reviewOnly": t["reviewOnly"],
                       "inputSchema": t["inputSchema"]} for t in sorted(TOOLS.values(), key=lambda t: t["name"])]}


def write_tool_catalog(path=SCHEMA_FILE):
    with open(path, "w", encoding="utf-8") as fh:
        json.dump(tool_catalog(), fh, ensure_ascii=False, indent=1, sort_keys=True)
        fh.write("\n")
    return path

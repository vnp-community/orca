"""File cache of LLM responses keyed by a hash of (provider+model, system, schema, user). Atomic writes."""
import hashlib
import json
import os
import tempfile


def cache_key(tag, system, schema, user):
    h = hashlib.sha256()
    for part in (tag, system, json.dumps(schema, sort_keys=True, ensure_ascii=False) if schema else "", user):
        h.update(part.encode("utf-8"))
        h.update(b"\x00")
    return h.hexdigest()


class FileCache:
    def __init__(self, root):
        self.root = os.path.join(root, "llm")

    def _path(self, key):
        return os.path.join(self.root, key[:2], key + ".json")

    def get(self, key):
        try:
            with open(self._path(key), encoding="utf-8") as fh:
                return json.load(fh)["value"]
        except (OSError, ValueError, KeyError):
            return None

    def put(self, key, value):
        path = self._path(key)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        fd, tmp = tempfile.mkstemp(dir=os.path.dirname(path), suffix=".tmp")
        try:
            with os.fdopen(fd, "w", encoding="utf-8") as fh:
                json.dump({"value": value}, fh, ensure_ascii=False)
            os.replace(tmp, path)
        except BaseException:
            if os.path.exists(tmp):
                os.unlink(tmp)
            raise

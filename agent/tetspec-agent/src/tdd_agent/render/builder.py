"""Rules that turn a knowledge-graph projection into the standard TDD document model.

Everything here is deterministic and uses no language model.  A statement appears only if it is in the
graph (every row names its source or the rule that derived it); what the graph lacks is written as a
:class:`Missing` block with the reason, never filled in.  The text is written for a tech lead: codes that only
the knowledge graph knows (``FACT-...``, ``TR-...``, ``SM-...``, ``ENT-01:attribute:x``) are never shown, the
source of a row is the section of the source document ("SRS §5.1"), and codes that come from the source
documents (``FR-ADM-01``, ``ENT-01``, ``ADR-03``) are kept and explained in appendix A.
"""
from __future__ import annotations

import re
from collections import Counter, defaultdict
from dataclasses import dataclass, field

from .model import (Bullets, Chapter, Document, Link, Missing, Note, ORIGIN_DERIVED, ORIGIN_SOURCE, Para, Row, Section, SourceRef, Table, UserText, walk_blocks)
from .rules import (CHILD_PARENT, SECTION_NUMBER, GOAL_TEXT, TECHNICAL_PROPS, clean, heading_section_number, id_family, is_family_row, is_hidden_code, last_heading, natural_key,
                    non_goal_passages, place_elements, pointers_for, prop_text, props, section_number, source_headings, strip_numbering)
from .template import CHAPTERS, NUMBER_OF, STRINGS, chapter_title

OWNER_BA = "BA / chủ sản phẩm"
TRACE_RELS = ("traces_to", "satisfies")
PENDING = re.compile(r"đề xuất|chờ|draft|chưa chốt|pending|tbd", re.I)
READINESS = {"blocked": "chưa sẵn sàng (blocked): còn thông tin bắt buộc bị thiếu hoặc chưa rõ", "ready": "sẵn sàng (ready)", "draft": "bản nháp (draft)"}
MATURITY = {"sketch": "bản phác", "needs_review": "chờ rà soát"}
NEED_NAMES = {"definition": ("Mã được nhắc tới nhưng không có mô tả", "ví dụ API-P-01 được nhắc ở một yêu cầu nhưng không tài liệu nào mô tả nó"),
              "class_mapping": ("Mã chưa biết thuộc loại phần tử nào", "nguồn có hàng với mã này nhưng chưa có luật gán loại"),
              "missing_recovery_objective": ("Yêu cầu khôi phục chưa nêu RTO/RPO", "thiếu con số mục tiêu khôi phục"),
              "prose_extraction": ("Văn xuôi chưa đọc hết", "một phần văn xuôi vượt giới hạn đọc"),
              "typed_fact": ("Sự kiện do mô hình đề xuất bị loại", "không kiểm chứng được với nguồn nên không đưa vào")}
NO_DATA = "chưa có trong dữ liệu đã trích từ tài liệu nguồn"


@dataclass
class RenderInfo:
    """What the graph alone does not say about this document."""
    tdd_id: str
    revision: str = ""
    snapshot_id: str = ""
    snapshot_hash: str = ""
    readiness: str = ""
    language: str = "vi"
    source_detail: str = "section"  # section = "SRS §5.1" | line = "SRS@1.0 d.463"
    source_texts: dict = field(default_factory=dict)  # (source id, revision) -> text, for headings and non-goal passages


def refs_of(provenance: list[dict] | None) -> list[SourceRef]:
    seen, out = set(), []
    for p in provenance or []:
        r = SourceRef(str(p.get("source_id") or p.get("source") or "?"), str(p.get("source_version") or p.get("revision") or "?"), str(p.get("block") or ""))
        if r not in seen:
            seen.add(r)
            out.append(r)
    return sorted(out, key=lambda r: (r.doc, r.revision, natural_key(r.block)))


def element_refs(e: dict) -> list[SourceRef]:
    prov = list(e.get("provenance") or [])
    for v in e.get("variants") or []:
        prov += v.get("provenance") or []
    return refs_of(prov)


def pairs(e: dict, skip: set[str] = frozenset(), with_variants: bool = True) -> list[str]:
    """``Label: value`` lines from the original source columns (or the descriptive props), skipping the id and repeated name."""
    name = clean(e.get("name"))
    out: list[str] = []

    def one(item: dict, prefix: str = "") -> None:
        cols = props(item).get("source_columns")
        shown = []
        if isinstance(cols, dict):
            for k, v in cols.items():
                val = clean(v)
                if not val or val == item["id"] or val == name or k in skip:
                    continue
                shown.append(f"{k}: {val}")
        else:
            for k, v in props(item).items():
                val = clean(v)
                if k in TECHNICAL_PROPS or k in skip or not val or val == name or k.startswith("_"):
                    continue
                shown.append(f"{k}: {val}")
        if shown and prefix:
            out.append(prefix)
        out.extend(shown)

    one(e)
    if with_variants:
        for v in e.get("variants") or []:
            heading = strip_numbering(last_heading(v))
            one(v, f"({heading or 'bảng khác'})")
    return out


def lines_cell(lines: list[str]) -> str:
    return "\n".join(lines) if lines else "—"


def short(text: str, n: int = 110) -> str:
    text = " ".join(text.split())
    return text if len(text) <= n else text[:n - 1].rstrip() + "…"


class Builder:
    def __init__(self, graph: dict, info: RenderInfo):
        self.g, self.info = graph, info
        self.lang = info.language
        self.s = STRINGS[self.lang]
        self.els = sorted(graph.get("elements", []), key=lambda e: natural_key(e["id"]))
        self.by_id = {e["id"]: e for e in self.els}
        self.known = set(self.by_id)
        self.edges = graph.get("edges", [])
        self.gaps = list(graph.get("gaps", []))
        self.conflicts = list(graph.get("conflicts", []))
        self.placed = place_elements(self.els)
        self.notes = [e for e in self.els if e.get("class") == "Section" and str(e["id"]).startswith("NOTE-") and props(e).get("markdown")]
        for e in self.notes:   # a person's text for one section: written where the person asked, never listed among the unplaced elements
            self.placed[e["id"]] = "notes"
        self.texts = info.source_texts
        self.shown: dict[str, str] = {}  # element id -> part key where it is written
        self.parts: dict[str, Chapter] = {}
        self._headings: dict[tuple[str, str], list] = {}

    # ---- helpers
    def part(self, key: str) -> Chapter:
        return self.parts[key]

    def num(self, key: str) -> str:
        return NUMBER_OF[key]

    def in_part(self, key: str, *classes: str) -> list[dict]:
        return [e for e in self.els if self.placed.get(e["id"]) == key and (not classes or e.get("class") in classes)]

    def mark(self, key: str, elements) -> None:
        for e in elements:
            self.shown[e["id"]] = key

    def title_of(self, e: dict) -> str:
        return clean(e.get("name")) or e["id"]

    def body_of(self, e: dict) -> str:
        """One readable sentence for an element: 'name: statement' when they differ."""
        t, b = self.title_of(e), prop_text(e)
        return b if b == t or not t or is_hidden_code(t) else f"{t}: {b}"

    def code(self, e: dict) -> str:
        return "" if is_hidden_code(e["id"]) else e["id"]

    def human(self, text: str) -> str:
        """Replace codes that only the knowledge graph knows (``FACT-ab12``) by the element's name."""
        def sub(m: re.Match) -> str:
            e = self.by_id.get(m.group(0))
            return f"“{short(self.title_of(e), 60)}”" if e else "(nội dung nêu trong văn xuôi)"
        return re.sub(r"FACT-[0-9a-f]{6,}", sub, text)

    def link(self, eid: str) -> Link | str:
        if eid not in self.known:
            return eid
        e = self.by_id[eid]
        return Link(eid, short(self.title_of(e), 48)) if is_hidden_code(eid) else Link(eid)

    def ids_cell(self, ids, unknown_mark: str = "") -> list:
        out: list = []
        for i, x in enumerate(sorted(set(ids), key=natural_key)):
            if i:
                out.append(", ")
            out.append(self.link(x) if x in self.known else x + unknown_mark)
        return out or ["—"]

    def link_tokens(self, text: str) -> list:
        out: list = []
        for piece in re.split(r"([A-Za-z]+(?:-[A-Za-z0-9]+)+)", text):
            out.append(self.link(piece) if piece in self.known else piece)
        return out

    def heading_at(self, doc: str, rev: str, line: int) -> str:
        """Number of the nearest numbered heading at or above ``line`` of a source document ('' when unknown)."""
        key = (doc, rev)
        if key not in self._headings:
            self._headings[key] = [(ln, m.group(1)) for ln, lvl, title in source_headings(self.texts.get(key, "")) if lvl >= 2 and (m := SECTION_NUMBER.match(title))]
        best = ""
        for ln, number in self._headings[key]:
            if ln <= line:
                best = number
            else:
                break
        return best

    def source_label(self, e: dict) -> str | None:
        """'SRS §5.1': the section of each source document the element comes from (the line number is the audit detail).  None = let the writer print the references."""
        if self.info.source_detail == "line":
            return None
        refs = element_refs(e)
        owner = e.get("owner_doc") or (refs[0].doc if refs else "?")
        numbers: dict[str, str] = {}
        for item in [e] + list(e.get("variants") or []):
            n = section_number(item)
            if n:
                numbers.setdefault(item.get("owner_doc") or owner, n)
        docs = list(dict.fromkeys([owner] + [r.doc for r in refs]))
        labels = []
        for doc in docs:
            n = numbers.get(doc)
            if not n:
                first = next((r for r in refs if r.doc == doc and r.block.startswith("line:")), None)
                if first:
                    n = self.heading_at(first.doc, first.revision, int(first.block.split(":", 1)[1]))
            labels.append(f"{doc} §{n}" if n else str(doc))
        return ", ".join(labels[:3]) + (f" +{len(labels) - 3}" if len(labels) > 3 else "")

    def element_row(self, e: dict, cells: list, origin: str = ORIGIN_SOURCE) -> Row:
        return Row(cells=cells, anchor=e["id"], origin=origin, refs=element_refs(e), label=self.source_label(e))

    def table_of(self, header: list[str], elements: list[dict], cells_fn, with_code: bool | None = None) -> Table:
        """Rows of elements; a first column 'Mã' is added only when the elements carry codes known in the source documents."""
        coded = any(self.code(e) for e in elements) if with_code is None else with_code
        rows = [self.element_row(e, ([self.code(e) or "—"] if coded else []) + cells_fn(e)) for e in elements]
        return Table((["Mã"] if coded else []) + header, rows)

    def missing(self, what: str, reason: str, topic: str | None = None, owner: str = "") -> Missing:
        ptr = []
        if topic:
            for key, line, title in pointers_for(topic, self.texts):
                n = heading_section_number(self.texts, key, line)
                ptr.append((f"{key[0]} §{n}" if n else f"{key[0]} (dòng {line})", strip_numbering(title)))
        return Missing(what, reason, owner, ptr)

    def gap_groups(self, need: str) -> dict[str, list[dict]]:
        out: dict[str, list[dict]] = defaultdict(list)
        for gap in self.gaps:
            if gap.get("need") == need:
                out[id_family(str(gap.get("target", "?")))].append(gap)
        return out

    def section(self, chapter: Chapter, title: str) -> Section:
        n = f"{chapter.number}.{sum(1 for b in chapter.blocks if isinstance(b, Section)) + 1}"
        sec = Section(number=n, title=title)
        chapter.blocks.append(sec)
        return sec

    # ---- build
    def build(self) -> Document:
        for spec in CHAPTERS:
            self.parts[spec.key] = Chapter(number=spec.number, title=chapter_title(spec, self.lang), anchor=f"ch-{spec.number}", key=spec.key)
        for fn in (self.p_scope, self.p_decomp, self.p_behavior, self.p_contracts, self.p_data, self.p_resilience, self.p_migration, self.p_quality, self.p_security,
                   self.p_trace, self.p_decisions, self.p_appendix):
            fn()
        self.apply_notes()
        front = self.front()
        return Document(title=self.s["title"], tdd_id=self.info.tdd_id, chapters=[self.parts[s.key] for s in CHAPTERS], front=front, language=self.lang,
                        meta={"revision": self.info.revision, "snapshot": self.info.snapshot_id, "snapshot_hash": self.info.snapshot_hash, "readiness": self.info.readiness})

    def apply_notes(self) -> None:
        """Put each person-written section text where it was asked: ``replace`` drops what the rules wrote in that section (its sub-sections stay), ``append`` puts the text first
        and only removes the notice that the section is missing."""
        def find(node, number):
            for b in node.blocks:
                if isinstance(b, Section):
                    if b.number == number:
                        return b
                    got = find(b, number)
                    if got:
                        return got
            return None
        orphans = None
        for e in sorted(self.notes, key=lambda x: natural_key(x["id"])):
            p = props(e)
            target = next((s for ch in self.parts.values() if (s := find(ch, str(p.get("section"))))), None)
            self.shown[e["id"]] = "notes"
            if target is None:   # the section is not in this layout (any more): the text is kept in the appendix, never dropped
                orphans = orphans or self.section(self.part("appendix"), "Nội dung người dùng ghi cho mục không còn trong tài liệu")
                orphans.blocks.append(Para(f"Mục {p.get('section')}:"))
                orphans.blocks.append(UserText(markdown=str(p.get("markdown") or ""), instruction=str(p.get("instruction") or ""), note_id=e["id"]))
                orphans.also_anchors = list(orphans.also_anchors) + [e["id"]]
                continue
            keep_sections = [b for b in target.blocks if isinstance(b, Section)]
            others = [b for b in target.blocks if not isinstance(b, Section)]
            if p.get("mode") == "replace":
                others = []
            else:
                others = [b for b in others if not isinstance(b, Missing)]
            target.blocks = [UserText(markdown=str(p.get("markdown") or ""), instruction=str(p.get("instruction") or ""), note_id=e["id"])] + others + keep_sections
            target.also_anchors = list(target.also_anchors) + [e["id"]]

    # ---- 1. scope and sources
    def p_scope(self) -> None:
        c = self.part("scope")
        caps = self.in_part("scope", "Capability")
        reqs = self.in_part("scope", "Requirement", "RequirementVersion")
        cons = self.in_part("scope", "Constraint")
        self.mark("scope", caps + reqs + cons)
        goals = self.section(c, "Mục tiêu")
        facts = [e for e in reqs if e["id"].startswith("FACT-")]
        general = [e for e in facts if GOAL_TEXT.search(self.title_of(e))]
        if caps:
            goals.blocks.append(self.table_of(["Năng lực", "Mục đích và phạm vi"], caps, lambda e: [self.title_of(e), lines_cell([f"{k}: {clean(v)}" for k, v in props(e).items() if k in ("purpose", "scope") and clean(v)])]))
        objectives = [e for e in reqs if re.match(r"BO-", e["id"])]
        if objectives:
            goals.blocks.append(Para("Mục tiêu kinh doanh (từ tài liệu nguồn):"))
            goals.blocks.append(self.table_of(["Mục tiêu", "Chỉ số"], objectives, lambda e: [self.title_of(e), clean((props(e).get("definition") or ["", ""])[1] if len(props(e).get("definition") or []) > 1 else "") or "—"]))
        if general:
            goals.blocks.append(Para("Mục tiêu tổng quát (câu mở đầu của tài liệu nguồn):"))
            goals.blocks.append(self.table_of(["Nội dung"], general, lambda e: [prop_text(e)], with_code=False))
        if not caps and not general and not objectives:
            goals.blocks.append(self.missing("Mục tiêu của hệ thống", f"{NO_DATA}.", "context"))
        fr = self.section(c, "Yêu cầu chức năng")
        functional = [e for e in reqs if not re.match(r"BO-", e["id"])]   # business objectives are listed in 1.1, not as functional requirements
        fam_title: dict[str, Counter] = defaultdict(Counter)
        for e in functional:
            if not e["id"].startswith("FACT-") and f"({id_family(e['id'])})" in last_heading(e):
                fam_title[id_family(e["id"])][last_heading(e)] += 1
        groups: dict[str, list[dict]] = defaultdict(list)
        for e in functional:
            if e["id"].startswith("FACT-"):
                if e not in general:
                    groups["99. Yêu cầu nêu trong văn xuôi (không nằm trong bảng yêu cầu)"].append(e)
            else:
                fam = id_family(e["id"])
                groups[fam_title[fam].most_common(1)[0][0] if fam_title.get(fam) else (last_heading(e) or fam)].append(e)
        if not groups:
            fr.blocks.append(self.missing("Yêu cầu chức năng", f"{NO_DATA}.", None))
        for heading in sorted(groups, key=natural_key):
            sub = Section(number=f"{fr.number}.{sum(1 for b in fr.blocks if isinstance(b, Section)) + 1}", title=f"{strip_numbering(heading)} ({len(groups[heading])})")
            sub.blocks.append(self.table_of(["Yêu cầu"], groups[heading], lambda e: [self.body_of(e) if e["id"].startswith("FACT-") else (prop_text(e) if self.title_of(e) == e["id"] else self.title_of(e))]))
            fr.blocks.append(sub)
        ng = self.section(c, "Phi mục tiêu")
        passages = non_goal_passages(self.texts)
        ng.blocks.append(self.missing(
            "Phi mục tiêu (những gì hệ thống cố ý không làm)",
            "Chưa có phần tử phi mục tiêu trong dữ liệu đã trích, nên tài liệu không tự suy diễn. Cần BA / chủ sản phẩm xác nhận; "
            + ("tài liệu nguồn có đoạn 'ngoài phạm vi' (trích nguyên văn bên dưới, chưa được đưa vào dữ liệu thiết kế)." if passages else "tài liệu nguồn không có mục 'ngoài phạm vi'."), None, OWNER_BA))
        for (doc, rev), line, body in passages:
            n = self.heading_at(doc, rev, line)
            where = f"{doc} §{n}, dòng {line}" if n else f"{doc}, dòng {line}"
            ng.blocks.append(Para(f"Trích nguyên văn từ {where}: " + re.sub(r"^[-*]\s+|\*\*", "", body[0]).strip()))
            if len(body) > 1:
                ng.blocks.append(Bullets([re.sub(r"^\s*([-*]|\d+[.)])\s+", "", b) for b in body[1:]]))
        cs = self.section(c, "Ràng buộc")
        if cons:
            cs.blocks.append(self.table_of(["Ràng buộc", "Nguyên văn nguồn"], cons, lambda e: [self.body_of(e), clean(props(e).get("evidence_span")) or "—"]))
        else:
            cs.blocks.append(self.missing("Ràng buộc chung từ SAD", f"{NO_DATA}.", "context"))
        ctx = self.section(c, "Bối cảnh hệ thống")
        ctx.blocks.append(self.missing("Actor, hệ thống ngoài, phụ thuộc bên ngoài", f"{NO_DATA}.", "context"))

    # ---- 2. decomposition
    def p_decomp(self) -> None:
        c = self.part("decomp")
        comps = self.in_part("decomp", "Component")
        mods = self.in_part("decomp", "Module")
        cons = self.in_part("decomp", "Constraint")
        self.mark("decomp", comps + mods + cons)
        a = self.section(c, "Container và thành phần chạy")
        rows = sorted(cons + comps, key=lambda e: natural_key(e["id"]))
        if rows:
            a.blocks.append(self.table_of(["Thành phần", "Trách nhiệm, giao tiếp, kho dữ liệu"], rows, lambda e: [self.title_of(e) if not is_hidden_code(e["id"]) else self.body_of(e), lines_cell(pairs(e, skip={"Container"}))]))
        else:
            a.blocks.append(self.missing("Container và thành phần", f"{NO_DATA}.", "components"))
        m = self.section(c, "Module của dịch vụ lõi")
        if mods:
            m.blocks.append(self.table_of(["Module", "Chi tiết (gồm quy tắc phụ thuộc giữa module)"], mods, lambda e: [self.title_of(e), lines_cell(pairs(e, skip={"Module (schema)"}))]))
        else:
            m.blocks.append(self.missing("Module", f"{NO_DATA}.", "components"))
        d = self.section(c, "Phụ thuộc giữa các thành phần")
        deps = [ed for ed in self.edges if ed.get("rel") == "depends_on"]
        if deps:
            d.blocks.append(Table(["Từ", "Phụ thuộc vào"], [Row([self.link(ed["src"]), self.link(ed["dst"])], origin=ORIGIN_SOURCE, refs=refs_of(ed.get("provenance"))) for ed in sorted(deps, key=lambda x: natural_key(x["src"] + x["dst"]))]))
        else:
            d.blocks.append(self.missing("Sơ đồ và quan hệ phụ thuộc giữa các thành phần", "Chưa có quan hệ phụ thuộc dạng có cấu trúc; quy tắc phụ thuộc chỉ có dạng chữ trong bảng module (nếu nguồn có).", "components"))
        al = self.section(c, "Phân bổ triển khai")
        al.blocks.append(self.missing("Thành phần nào chạy ở đâu (môi trường, mạng, mức triển khai)", f"{NO_DATA}.", "deploy"))

    # ---- 3. behavior
    def p_behavior(self) -> None:
        c = self.part("behavior")
        machines = self.in_part("behavior", "StateMachine")
        states = self.in_part("behavior", "State")
        trans = self.in_part("behavior", "Transition")
        rules = self.in_part("behavior", "BusinessRule")
        rest = [e for e in self.in_part("behavior") if e.get("class") not in ("StateMachine", "State", "Transition", "BusinessRule")]
        self.mark("behavior", machines + states + trans + rules + rest)
        a = self.section(c, "Vòng đời trạng thái")
        by_t: dict[str, list[dict]] = defaultdict(list)
        by_s: dict[str, list[dict]] = defaultdict(list)
        for t in trans:
            by_t[t["id"].split(":")[0].removeprefix("TR-")].append(t)
        for s in states:
            by_s[props(s).get("machine") or s["id"].split(":")[0]].append(s)
        if machines:
            a.blocks.append(Para(f"{len(machines)} thực thể có vòng đời trạng thái trong tài liệu nguồn."))
        for m in machines:
            owner = self.by_id.get(props(m).get("owner_entity") or "")
            title = f"{self.title_of(owner)} ({owner['id']})" if owner else self.title_of(m)
            st = sorted(by_s.get(m["id"], []), key=lambda x: natural_key(x["id"]))
            sub = Section(number=f"{a.number}.{sum(1 for b in a.blocks if isinstance(b, Section)) + 1}", title=f"Vòng đời của {title}", also_anchors=[m["id"]] + [x["id"] for x in st])
            if st:
                sub.blocks.append(Para("Trạng thái: " + ", ".join(clean(x.get("name")) for x in st) + "."))
            ts = sorted(by_t.get(m["id"], []), key=lambda t: natural_key(t["id"]))
            if ts:
                sub.blocks.append(Table(["Từ", "Đến", "Sự kiện", "Điều kiện", "Tác động kèm theo"],
                                        [Row([clean(props(t).get("from")), clean(props(t).get("to")), clean(props(t).get("event")) or "—", clean(props(t).get("guard")) or "—", clean(props(t).get("side_effects")) or "—"],
                                             anchor=t["id"], origin=ORIGIN_SOURCE, refs=element_refs(t), label=self.source_label(t)) for t in ts]))
            else:
                sub.blocks.append(self.missing(f"Chuyển trạng thái của {title}", f"{NO_DATA}.", None))
            a.blocks.append(sub)
        if not machines:
            a.blocks.append(self.missing("Máy trạng thái", f"{NO_DATA}.", "flows"))
        b = self.section(c, "Quy tắc nghiệp vụ")
        if rules:
            b.blocks.append(self.table_of(["Quy tắc", "Liên quan (yêu cầu, thuật toán, cơ chế thực thi)"], rules, lambda e: [self.body_of(e) if is_hidden_code(e["id"]) else self.title_of(e), lines_cell(pairs(e, skip={"Nội dung"}))]))
        else:
            b.blocks.append(self.missing("Quy tắc nghiệp vụ", f"{NO_DATA}.", "flows"))
        if rest:
            b.blocks.append(self.table_of(["Phần tử", "Chi tiết"], rest, lambda e: [self.title_of(e), lines_cell(pairs(e))]))
        d = self.section(c, "Luồng xử lý, thuật toán, bảng quyết định")
        alg = self.gap_groups("definition").get("ALG", [])
        d.blocks.append(self.missing("Luồng chính/phụ/lỗi, thuật toán, bảng quyết định, bất biến",
                                     (f"{NO_DATA}; {len({g['target'] for g in alg})} thuật toán (mã ALG-…) được nhắc tới nhưng chưa có mô tả (xem phụ lục A.2)." if alg else f"{NO_DATA}."), "flows"))

    # ---- 4. interface contracts
    def p_contracts(self) -> None:
        c = self.part("contracts")
        ops = self.in_part("contracts")
        self.mark("contracts", ops)
        a = self.section(c, "Giao diện đã có mô tả")
        if ops:
            a.blocks.append(self.table_of(["Giao diện", "Chi tiết"], ops, lambda e: [self.title_of(e), lines_cell(pairs(e))]))
        else:
            a.blocks.append(Para("Chưa có giao diện nào được mô tả đầy đủ trong dữ liệu đã trích."))
        b = self.section(c, "Giao diện được nhắc tới nhưng chưa có hợp đồng")
        groups = {k: v for k, v in self.gap_groups("definition").items() if k.split("-")[0] in ("API", "EIF", "WH", "EVT")}
        if groups:
            b.blocks.append(Table(["Nhóm mã", "Số mã được nhắc", "Ví dụ (mã ← nhắc ở)"], [Row([k, str(len({g['target'] for g in v})), ", ".join(f"{g['target']} ← {self.human(str(g.get('referenced_by', '?')))}" for g in sorted(v, key=lambda g: natural_key(g['target']))[:3])], origin=ORIGIN_DERIVED) for k, v in sorted(groups.items(), key=lambda kv: natural_key(kv[0]))]))
        b.blocks.append(self.missing("Hợp đồng của từng endpoint (đường dẫn, phương thức, đầu vào/ra, mã lỗi, xác thực, phiên bản, tương thích ngược, bên tiêu thụ)",
                                     "Nguồn mới nêu mã giao diện, chưa có hợp đồng chi tiết trong dữ liệu đã trích.", "api"))

    # ---- 5. data
    def p_data(self) -> None:
        c = self.part("data")
        ents = self.in_part("data", "Entity")
        attrs = self.in_part("data", "Attribute")
        ints = self.in_part("data", "Constraint")
        others = [e for e in self.in_part("data") if e.get("class") not in ("Entity", "Attribute", "Constraint")]
        self.mark("data", ents + attrs + ints + others)
        by_ent: dict[str, list[dict]] = defaultdict(list)
        for a in attrs:
            by_ent[props(a).get("entity") or a["id"].split(":")[0]].append(a)
        s1 = self.section(c, "Thực thể")
        if ents:
            def cells(e: dict) -> list:
                u = props(e).get("upstream_definition") or {}
                cols = props(e).get("source_columns") or {}
                return [self.title_of(e), clean(u.get("m_t")) or "—", clean(u.get("nh_danh")) or "—", clean(cols.get("Đọc")) or "—", clean(cols.get("Ghi")) or "—", clean(cols.get("Ghi chú")) or "—"]
            s1.blocks.append(self.table_of(["Thực thể", "Mô tả", "Định danh", "API đọc", "API ghi", "Ghi chú"], ents, cells))
        else:
            s1.blocks.append(self.missing("Thực thể", f"{NO_DATA}.", "data"))
        s2 = self.section(c, "Thuộc tính của từng thực thể")
        unknown = sum(1 for a in attrs if clean(props(a).get("physical_type")) in ("", "unknown"))
        s2.blocks.append(Para(f"Mức mô tả: logic. Kiểu vật lý {NO_DATA} ở {unknown}/{len(attrs)} thuộc tính (đánh dấu **THIẾU**). Cột 'Khoá' đánh dấu thuộc tính trùng tên với phần 'định danh' của thực thể trong nguồn."))
        for e in ents:
            sub = Section(number=f"{s2.number}.{sum(1 for b in s2.blocks if isinstance(b, Section)) + 1}", title=f"{self.title_of(e)} ({e['id']})")
            key_names = set(re.findall(r"[A-Za-z_][A-Za-z0-9_]*", clean((props(e).get("upstream_definition") or {}).get("nh_danh"))))
            mine = sorted(by_ent.get(e["id"], []), key=lambda a: natural_key(a["id"]))
            if mine:
                sub.blocks.append(Table(["Thuộc tính", "Khoá", "Kiểu vật lý"], [Row([a["name"], "định danh theo nguồn" if a["name"] in key_names else "—", (clean(props(a).get("physical_type")) if clean(props(a).get("physical_type")) not in ("", "unknown") else "**THIẾU**")],
                                                                                     anchor=a["id"], origin=ORIGIN_SOURCE, refs=element_refs(a), label=self.source_label(a)) for a in mine]))
            else:
                sub.blocks.append(self.missing(f"Thuộc tính của {self.title_of(e)}", f"{NO_DATA}.", None))
            s2.blocks.append(sub)
        s3 = self.section(c, "Toàn vẹn dữ liệu")
        if ints:
            s3.blocks.append(self.table_of(["Ràng buộc", "Cơ chế thực thi và nguồn gốc"], ints, lambda e: [self.title_of(e), lines_cell(pairs(e, skip={"Quy tắc URD"})) if pairs(e) else prop_text(e)]))
        else:
            s3.blocks.append(self.missing("Ràng buộc toàn vẹn dữ liệu", f"{NO_DATA}.", "data"))
        if others:
            s3.blocks.append(Para("Quy ước dữ liệu nêu trong văn xuôi:"))
            s3.blocks.append(self.table_of(["Quy ước"], others, lambda e: [self.body_of(e)], with_code=False))
        s4 = self.section(c, "Thông tin dữ liệu còn thiếu")
        s4.blocks.append(self.missing("Kiểu dữ liệu vật lý, độ dài, nullability, giá trị mặc định", f"{unknown}/{len(attrs)} thuộc tính chưa có kiểu vật lý.", "data"))
        s4.blocks.append(self.missing("Khoá ngoại, chỉ mục, view, phân vùng, phân loại dữ liệu, retention, ownership", f"{NO_DATA}.", "data"))

    # ---- 6. transactions and resilience
    def p_resilience(self) -> None:
        c = self.part("resilience")
        errs = self.in_part("resilience", "ErrorCode")
        pol = [e for e in self.in_part("resilience") if e.get("class") != "ErrorCode"]
        self.mark("resilience", errs + pol)
        a = self.section(c, "Mã lỗi")
        if errs:
            a.blocks.append(self.table_of(["Mã lỗi", "HTTP", "Ý nghĩa"], errs, lambda e: [self.title_of(e), clean(props(e).get("http_status")) or "—", clean(props(e).get("meaning")) or "—"], with_code=False))
        else:
            a.blocks.append(self.missing("Mã lỗi", f"{NO_DATA}.", "failure"))
        b = self.section(c, "Chính sách chịu lỗi nêu trong nguồn")
        if pol:
            b.blocks.append(self.table_of(["Nội dung", "Chi tiết"], pol, lambda e: [self.body_of(e), lines_cell(pairs(e, skip={"statement"})) if not is_hidden_code(e["id"]) else (clean(props(e).get("evidence_span")) or "—")]))
        b.blocks.append(self.missing("Ranh giới transaction, isolation, nhất quán, saga/bù trừ, retry/backoff, timeout, idempotency, fallback, DLQ", f"{NO_DATA}.", "failure"))

    # ---- 7. migration
    def p_migration(self) -> None:
        c = self.part("migration")
        items = self.in_part("migration")
        self.mark("migration", items)
        a = self.section(c, "Di trú nêu trong nguồn")
        if items:
            a.blocks.append(self.table_of(["Nội dung", "Chi tiết"], items, lambda e: [self.body_of(e), lines_cell(pairs(e))]))
        else:
            a.blocks.append(Para("Dữ liệu đã trích chưa có mục di trú nào."))
        b = self.section(c, "Thông tin di trú còn thiếu")
        b.blocks.append(self.missing("Schema trước/sau, các bước expand → migrate → contract, backfill, khoá bảng, downtime, tương thích, rollback", f"{NO_DATA}.", "migration"))

    # ---- 8 and 9. quality / operations, security
    def items_part(self, key: str, title: str, missing_items: list[tuple[str, str, str | None]]) -> None:
        c = self.part(key)
        items = [e for e in self.in_part(key) if e["id"] not in self.shown]
        self.mark(key, items)
        a = self.section(c, title)
        groups: dict[str, list[dict]] = defaultdict(list)
        for e in items:
            groups[last_heading(e) or "Nêu trong văn xuôi của tài liệu nguồn"].append(e)
        if not items:
            a.blocks.append(Para("Dữ liệu đã trích chưa có mục nào xếp vào phần này."))
        for heading in sorted(groups, key=natural_key):
            sub = Section(number=f"{a.number}.{sum(1 for b in a.blocks if isinstance(b, Section)) + 1}", title=f"{strip_numbering(heading)} ({len(groups[heading])})")
            sub.blocks.append(self.table_of(["Yêu cầu", "Chỉ tiêu và chi tiết"], sorted(groups[heading], key=lambda e: natural_key(e["id"])),
                                            lambda e: [self.title_of(e) if not is_hidden_code(e["id"]) else self.body_of(e), lines_cell(pairs(e, skip={"Yêu cầu"})) if not is_hidden_code(e["id"]) else (clean(props(e).get("evidence_span")) or "—")]))
            a.blocks.append(sub)
        gaps = self.section(c, "Thông tin còn thiếu")
        for what, reason, topic in missing_items:
            gaps.blocks.append(self.missing(what, reason, topic))

    def p_quality(self) -> None:
        rec = self.gap_groups("missing_recovery_objective")
        n = sum(len(v) for v in rec.values())
        items = [("Mô hình tải (trung bình, đỉnh, payload, tăng trưởng), capacity, SLO, concurrency, cấu hình sẵn sàng cao", f"{NO_DATA}; các con số quy mô chỉ xuất hiện trong chỉ tiêu ở trên.", None),
                 ("Kiến trúc triển khai (môi trường, mạng, mức triển khai) và kế hoạch phát hành", f"{NO_DATA}.", "deploy"),
                 ("Quan sát: log, metric, trace, cảnh báo", f"{NO_DATA}.", "observability")]
        if n:
            targets = sorted({g["target"] for v in rec.values() for g in v}, key=natural_key)
            items.insert(0, ("Mục tiêu khôi phục (RTO/RPO)", f"{n} yêu cầu nhắc đến khôi phục nhưng chưa nêu RTO/RPO: " + ", ".join(self.title_of(self.by_id[t]) if is_hidden_code(t) and t in self.by_id else t for t in targets) + ".", "failure"))
        self.items_part("quality", "Chỉ tiêu và ràng buộc chất lượng, vận hành", items)

    def p_security(self) -> None:
        self.items_part("security", "Yêu cầu và ràng buộc bảo mật đã có", [
            ("Ranh giới tin cậy và ma trận phân quyền (vai trò × thao tác)", f"{NO_DATA}.", "security"),
            ("Mô hình mối đe doạ và biện pháp kiểm soát", f"{NO_DATA}.", "security"),
            ("Vòng đời credential, bảo vệ dữ liệu, kiểm toán", f"{NO_DATA}.", "security")])

    # ---- 11. verification and traceability
    def p_trace(self) -> None:
        c = self.part("trace")
        tcs = self.in_part("trace", "TestCase", "Scenario", "TestReceipt")
        fam = [e for e in self.els if is_family_row(e)]
        self.mark("trace", tcs + fam)
        a = self.section(c, "Tiêu chí nghiệm thu gắn yêu cầu")
        with_acc, without = [], []
        for e in self.els:
            if e.get("class") in ("Requirement", "QualityTarget") and not e["id"].startswith("FACT-") and not is_family_row(e):
                acc = clean(props(e).get("acceptance")) or clean(props(e).get("nghi_m_thu"))
                (with_acc if acc else without).append((e, acc))
        if with_acc:
            a.blocks.append(Table(["Yêu cầu", "Tiêu chí nghiệm thu (nguyên văn nguồn)"], [Row([self.link(e["id"]), acc], origin=ORIGIN_SOURCE, refs=element_refs(e), label=self.source_label(e)) for e, acc in with_acc]))
        a.blocks.append(Para(f"{len(with_acc)}/{len(with_acc) + len(without)} yêu cầu có tiêu chí nghiệm thu trong dữ liệu đã trích."))
        if without:
            a.blocks.append(Para("Chưa có tiêu chí nghiệm thu: " + ", ".join(e["id"] for e, _ in without[:60]) + (f" … (+{len(without) - 60})" if len(without) > 60 else "") + "."))
        b = self.section(c, "Test case, kịch bản và nghĩa vụ thiết kế")
        if tcs:
            b.blocks.append(self.table_of(["Mục", "Chi tiết"], tcs, lambda e: [self.title_of(e), lines_cell(pairs(e))]))
        else:
            b.blocks.append(self.missing("Test case, kịch bản kiểm thử, kết quả mong đợi, nghĩa vụ thiết kế (design obligation)", f"{NO_DATA}.", "test"))
        # traceability from the graph edges
        adj: dict[str, dict[str, list[dict]]] = defaultdict(lambda: defaultdict(list))
        for ed in self.edges:
            if ed.get("rel") in TRACE_RELS:
                adj[ed["src"]]["out"].append(ed)
                adj[ed["dst"]]["in"].append(ed)
        undefined: dict[str, set[str]] = defaultdict(set)
        for gap in self.gaps:
            if gap.get("need") == "definition" and gap.get("referenced_by"):
                undefined[gap["referenced_by"]].add(str(gap["target"]))
        cols = [("BusinessRule", "Quy tắc nghiệp vụ"), ("Entity", "Thực thể"), ("QualityTarget", "Phi chức năng"), ("Requirement", "Yêu cầu liên quan"), ("arch", "Kiến trúc, ràng buộc, quyết định")]
        arch = {"Component", "Module", "Constraint", "Decision"}

        def col_of(e: dict | None) -> str:
            cls = (e or {}).get("class")
            return "arch" if cls in arch else cls if cls in {k for k, _ in cols} else "other"

        def trace_table(sources: list[dict]) -> Table:
            rows = []
            for e in sources:
                neigh: dict[str, set[str]] = defaultdict(set)
                docs: list[str] = []
                for d in ("out", "in"):
                    for ed in adj[e["id"]][d]:
                        other = ed["dst"] if d == "out" else ed["src"]
                        if other != e["id"]:
                            neigh[col_of(self.by_id.get(other))].add(other)
                            docs += [r.doc for r in refs_of(ed.get("provenance"))]
                cells = [self.link(e["id"])] + [self.ids_cell(neigh[k]) if neigh.get(k) else "—" for k, _ in cols]
                rest = sorted(neigh.get("other", set()) | undefined.get(e["id"], set()), key=natural_key)
                cells.append(", ".join(x + ("" if x in self.known else " *") for x in rest) if rest else "—")
                rows.append(Row(cells=cells, origin=ORIGIN_SOURCE if docs else ORIGIN_DERIVED, refs=element_refs(e), label=", ".join(dict.fromkeys(docs)) or self.source_label(e)))
            return Table(["Mã"] + [h for _, h in cols] + ["Khác (* = chưa có mô tả)"], rows)

        reqs = [e for e in self.els if e.get("class") == "Requirement" and not e["id"].startswith("FACT-") and not is_family_row(e)]
        nfrs = [e for e in self.els if e.get("class") == "QualityTarget" and not e["id"].startswith("FACT-") and not is_family_row(e)]
        t1 = self.section(c, "Yêu cầu chức năng → phần tử thiết kế")
        t1.blocks.append(trace_table(reqs) if reqs else self.missing("Truy vết yêu cầu chức năng", f"{NO_DATA}.", None))
        t2 = self.section(c, "Yêu cầu phi chức năng → phần tử thiết kế")
        t2.blocks.append(trace_table(nfrs) if nfrs else self.missing("Truy vết yêu cầu phi chức năng", f"{NO_DATA}.", None))
        t3 = self.section(c, "Nhóm yêu cầu → thành phần kiến trúc (bảng truy vết của SAD)")
        if fam:
            t3.blocks.append(Table(["Nhóm yêu cầu", "Thành phần kiến trúc"], [Row([e["id"], self.link_tokens(self.title_of(e))], anchor=e["id"], origin=ORIGIN_SOURCE, refs=element_refs(e), label=self.source_label(e)) for e in fam]))
        else:
            t3.blocks.append(self.missing("Bảng nhóm yêu cầu → thành phần kiến trúc", f"{NO_DATA}.", "components"))
        d = self.section(c, "Độ phủ truy vết")
        stats = []
        for label, group in (("Yêu cầu chức năng", reqs), ("Yêu cầu phi chức năng", nfrs)):
            def have(cls: str) -> int:
                return sum(1 for e in group if any(col_of(self.by_id.get(ed["dst"] if dd == "out" else ed["src"])) == cls for dd in ("out", "in") for ed in adj[e["id"]][dd]))
            acc = sum(1 for e in group if clean(props(e).get("acceptance")) or clean(props(e).get("nghi_m_thu")))
            stats.append(Row([label, str(len(group)), str(have("BusinessRule")), str(have("Entity")), str(have("arch")), str(acc), "0"], origin=ORIGIN_DERIVED))
        d.blocks.append(Table(["Nhóm", "Tổng", "Có quy tắc nghiệp vụ", "Có thực thể", "Có kiến trúc/ràng buộc", "Có tiêu chí nghiệm thu", "Có test case"], stats))
        d.blocks.append(Note("Tính từ các quan hệ truy vết trong dữ liệu đã trích. 'Có test case' bằng 0 vì chưa có test case nào được trích (xem 11.2)."))

    # ---- 10. decisions, assumptions, open gaps
    def p_decisions(self) -> None:
        c = self.part("decisions")
        decisions = self.in_part("decisions", "Decision")
        self.mark("decisions", decisions)
        adr = [e for e in decisions if not e["id"].startswith("FACT-")]
        facts = [e for e in decisions if e["id"].startswith("FACT-")]
        a = self.section(c, "Quyết định kiến trúc (ADR)")
        if adr:
            a.blocks.append(self.table_of(["Quyết định", "Lý do, phương án đã loại, hệ quả"], adr, lambda e: [prop_text(e) or self.title_of(e), lines_cell(pairs(e, skip={"Quyết định", "QA"}))]))
        else:
            a.blocks.append(self.missing("Quyết định kiến trúc (ADR)", f"{NO_DATA}.", "components"))
        b = self.section(c, "Quyết định nêu trong văn xuôi")
        if facts:
            b.blocks.append(Para("Mục đánh dấu **[chưa chốt]** có chữ 'đề xuất', 'chờ' hoặc 'draft' trong nguồn."))
            b.blocks.append(self.table_of(["Quyết định", "Tình trạng"], facts, lambda e: [self.body_of(e), "**[chưa chốt]**" if PENDING.search(clean(props(e).get("evidence_span")) + " " + prop_text(e) + " " + self.title_of(e)) else "—"], with_code=False))
        else:
            b.blocks.append(Para("Không có quyết định nào nêu trong văn xuôi."))
        g = self.section(c, "Giả định")
        g.blocks.append(self.missing("Giả định về tổ chức và hạ tầng", f"{NO_DATA}.", "context"))
        # gaps in plain language
        h = self.section(c, "Thông tin còn thiếu trong dữ liệu đã trích")
        by_need: dict[str, list[dict]] = defaultdict(list)
        for x in self.gaps:
            by_need[str(x.get("need"))].append(x)
        names = NEED_NAMES
        if by_need:
            h.blocks.append(Table(["Loại", "Số lượng", "Ý nghĩa"], [Row([names.get(k, (k, ""))[0], str(len(v)), names.get(k, ("", ""))[1]], origin=ORIGIN_DERIVED) for k, v in sorted(by_need.items())]))
        else:
            h.blocks.append(Para("Không ghi nhận thông tin thiếu."))
        defs = self.gap_groups("definition")
        if defs:
            h.blocks.append(Para("Mã được nhắc tới nhưng chưa có mô tả, theo nhóm (danh sách đầy đủ ở phụ lục A.2):"))
            h.blocks.append(Table(["Nhóm mã", "Số mã", "Ví dụ"], [Row([k, str(len({x['target'] for x in v})), ", ".join(sorted({x['target'] for x in v}, key=natural_key)[:6])], origin=ORIGIN_DERIVED) for k, v in sorted(defs.items(), key=lambda kv: natural_key(kv[0]))]))
        cm = by_need.get("class_mapping", [])
        if cm:
            fams = Counter(id_family(str(x["target"])) for x in cm)
            h.blocks.append(Para("Các nhóm mã chưa biết loại (số hàng): " + ", ".join(f"{k} ({v})" for k, v in sorted(fams.items())) + "."))
        i = self.section(c, "Mâu thuẫn giữa các nguồn")
        if self.conflicts:
            i.blocks.append(Table(["Mâu thuẫn", "Chi tiết"], [Row([clean(x.get("id") or x.get("target") or "?"), clean({k: v for k, v in x.items() if k != "id"})], origin=ORIGIN_SOURCE) for x in self.conflicts]))
        else:
            i.blocks.append(Para("Không ghi nhận mâu thuẫn."))
        r = self.section(c, "Rủi ro")
        r.blocks.append(self.missing("Danh mục rủi ro (khả năng, tác động, giảm thiểu), nợ kỹ thuật chủ động chấp nhận", f"{NO_DATA}.", "risks"))
        t = self.section(c, "Tổng hợp mục THIẾU của toàn tài liệu")
        rows = []
        for ch in self.parts.values():
            for sec, blk in walk_blocks(ch):
                if isinstance(blk, Missing):
                    rows.append(Row([f"{sec.number} {sec.title}", blk.what, blk.reason, blk.owner or "—"], origin=ORIGIN_DERIVED))
        t.blocks.append(Table(["Mục", "Còn thiếu", "Lý do", "Cần ai xác nhận"], rows) if rows else Para("Không có mục THIẾU."))

    # ---- appendix
    def p_appendix(self) -> None:
        c = self.part("appendix")
        unplaced = [e for e in self.els if e["id"] not in self.shown and self.placed.get(e["id"]) is None]
        self.mark("appendix", unplaced)
        a = self.section(c, "Bảng tra mã")
        a.blocks.append(Para("Các mã (FR-…, BRL-…, ENT-…, NFR-…, ADR-…, M-…, SVC-…) lấy nguyên từ tài liệu nguồn. Bảng này cho biết mã đó là gì và nằm ở phần nào của tài liệu này."))
        rows = []
        for e in self.els:
            if e["id"] in self.shown and not is_hidden_code(e["id"]) and e.get("class") not in CHILD_PARENT:
                name = self.title_of(e)
                name = prop_text(e) if name == e["id"] else name
                rows.append(Row([self.link(e["id"]), short(name), self.num(self.shown[e["id"]])], origin=ORIGIN_DERIVED))
        a.blocks.append(Table(["Mã", "Tên hoặc nội dung", "Phần"], rows) if rows else Para("Không có mã nào."))
        b = self.section(c, "Danh sách đầy đủ thông tin còn thiếu")
        if self.gaps:
            b.blocks.append(Table(["Mã được nhắc", "Loại", "Nhắc ở", "Vấn đề"], [Row([self.human(str(x.get("target", "—"))), NEED_NAMES.get(str(x.get("need")), (str(x.get("need")), ""))[0], self.human(str(x.get("referenced_by", "—"))), self.human(str(x.get("reason", "")))], origin=ORIGIN_DERIVED) for x in sorted(self.gaps, key=lambda x: natural_key(str(x.get("target")) + str(x.get("id"))))]))
        else:
            b.blocks.append(Para("Không ghi nhận thông tin thiếu."))
        u = self.section(c, "Phần tử chưa xếp vào phần nào")
        if unplaced:
            u.blocks.append(self.table_of(["Loại", "Tên", "Chi tiết"], unplaced, lambda e: [e.get("class", "?"), self.title_of(e), lines_cell(pairs(e))]))
        else:
            u.blocks.append(Para("Mọi phần tử đều đã có chỗ trong các phần 1–11."))

    # ---- front matter (built last: it summarizes the rest)
    def front(self) -> Section:
        i = self.info
        f = Section(number="", title=self.s["front"])
        a = Section(number="0.1", title="Thông tin tài liệu")
        maturity = Counter(f"{MATURITY.get(e.get('maturity'), e.get('maturity'))} / {MATURITY.get(e.get('review_status'), e.get('review_status'))}" for e in self.els)
        used: dict[tuple[str, str], str] = {}
        for e in self.els:
            for p in e.get("provenance") or []:
                key = (str(p.get("source_id")), str(p.get("source_version")))
                used[key] = used.get(key) or str(p.get("source_hash") or "")
        a.blocks.append(Table(["Mục", "Giá trị"], [
            Row(["Mã tài liệu", i.tdd_id], origin=ORIGIN_DERIVED),
            Row(["Phiên bản", i.revision or "—"], origin=ORIGIN_DERIVED),
            Row(["Chủ sở hữu, người duyệt, ngày duyệt", "**THIẾU** — dữ liệu đã trích không lưu thông tin này"], origin=ORIGIN_DERIVED),
            Row(["Tài liệu nguồn", "; ".join(f"{k[0]} v{k[1]}" for k in sorted(used, key=lambda k: natural_key(k[0] + k[1]))) or "—"], origin=ORIGIN_DERIVED),
            Row(["Trạng thái", READINESS.get(i.readiness, i.readiness or "—")], origin=ORIGIN_DERIVED),
            Row(["Mức chín", f"{'; '.join(f'{v} phần tử {k}' for k, v in sorted(maturity.items()))} — chưa được tech lead rà soát"], origin=ORIGIN_DERIVED),
            Row(["Quy mô dữ liệu thiết kế", f"{len(self.els)} phần tử, {len(self.edges)} quan hệ, {len(self.gaps)} chỗ còn thiếu, {len(self.conflicts)} mâu thuẫn"], origin=ORIGIN_DERIVED),
            Row(["Bản dữ liệu đã dùng để sinh tài liệu", f"{i.snapshot_id} · {i.snapshot_hash}" if i.snapshot_id else "—"], origin=ORIGIN_DERIVED),
        ]))
        f.blocks.append(a)
        b = Section(number="0.2", title="Tình trạng dữ liệu theo phần")
        rows = []
        for spec in CHAPTERS[:-1]:
            ch = self.parts[spec.key]
            miss = sum(1 for _, blk in walk_blocks(ch) if isinstance(blk, Missing))
            body_rows = sum(len(blk.rows) for _, blk in walk_blocks(ch) if isinstance(blk, Table))
            count = sum(1 for e in self.els if self.shown.get(e["id"]) == spec.key and e.get("class") not in CHILD_PARENT)
            status = "chưa có dữ liệu" if body_rows == 0 else "một phần (còn mục THIẾU)" if miss else "đủ theo dữ liệu đã trích"
            rows.append(Row([Link(f"ch-{spec.number}", f"{spec.number}. {ch.title}"), str(count), str(miss), status], origin=ORIGIN_DERIVED))
        b.blocks.append(Table(["Phần", "Số mục dữ liệu", "Mục THIẾU", "Tình trạng"], rows))
        f.blocks.append(b)
        c = Section(number="0.3", title="Cách đọc")
        c.blocks.append(Bullets([self.s["reader_note"],
                                 "Cột **Nguồn** ghi tài liệu và mục của tài liệu đó (ví dụ `SRS §5.1`); nội dung văn xuôi ghi số dòng. Số liệu do luật tính ra ghi 'tổng hợp' hoặc không có cột nguồn.",
                                 "Mã như FR-…, BRL-…, ENT-…, NFR-…, ADR-… lấy từ tài liệu nguồn; phụ lục A.1 tra mã. Mã nội bộ của hệ thống sinh dữ liệu không xuất hiện trong tài liệu.",
                                 "**THIẾU** = chưa có trong dữ liệu đã trích; không được điền bằng suy đoán. Tổng hợp ở mục 10.7."]))
        f.blocks.append(c)
        return f


def build_document(graph: dict, info: RenderInfo) -> Document:
    return Builder(graph, info).build()

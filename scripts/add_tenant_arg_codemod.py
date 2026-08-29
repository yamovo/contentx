"""One-shot codemod: add tenantID arg to ContentTypeService call sites in tests.

Rules (repo already migrated; only service-level test callers remain):
- Multi-line calls ending with a bare `})` line  -> `}, 1)`   (CreateContentType, ListEntries w/ params literal)
- Calls ending `}, user.ID)` (single or multi-line) -> `}, 1, user.ID)` (CreateEntry/UpdateEntry/Publish/Import/Translation)
- Single-line simple calls get `, 1` appended before the final `)`.
Only lines inside a tracked call are touched.
"""
import os
import re
import sys

FILES = [
    r"internal/services/content_service_test.go",
    r"internal/services/i18n_test.go",
    r"internal/services/constructors_test.go",
]

# Methods whose last arg is user.ID -> insert `1, ` before it.
USER_ID_METHODS = ("CreateEntry", "UpdateEntry", "PublishEntry", "UnpublishEntry", "ImportEntries", "CreateEntryTranslation")
# Methods ending with a bare `})` terminator line (request/params literal).
LITERAL_METHODS = ("CreateContentType", "ListEntries")
# Single-line: append `, 1` before closing paren.
APPEND_METHODS = (
    "GetContentType", "DeleteContentType", "ListContentTypes", "GetEntry",
    "DeleteEntry", "ExportEntries", "SearchEntries", "ListEntryTranslations",
    "GetEntriesByUID",
)

CALL_RE = re.compile(r"\b(?:svc|s)\.(" + "|".join(USER_ID_METHODS + LITERAL_METHODS + APPEND_METHODS) + r")\(")

def process(text: str) -> str:
    lines = text.split("\n")
    out = []
    pending = None  # (method, indent) while inside a multi-line call
    for line in lines:
        if pending is not None:
            method = pending
            stripped = line.rstrip()
            if method in USER_ID_METHODS and "}, user.ID)" in stripped:
                line = line.replace("}, user.ID)", "}, 1, user.ID)", 1)
                pending = None
            elif method in LITERAL_METHODS and stripped.lstrip().startswith("}") and "})" in stripped:
                idx = line.find("})")
                line = line[:idx] + "}, 1)" + line[idx + 2:]
                pending = None
            out.append(line)
            continue

        m = CALL_RE.search(line)
        already = ("tenantID" in line) or (", 1)" in line) or ("(1)" in line) or (", 1," in line)
        if m and not already:
            method = m.group(1)
            if method in USER_ID_METHODS:
                if "}, user.ID)" in line:
                    line = line.replace("}, user.ID)", "}, 1, user.ID)", 1)
                elif "Request{" in line:
                    pending = method
                elif line.rstrip().endswith(", user.ID)"):
                    # variable-arg form, e.g. PublishEntry("x", id, user.ID)
                    line = line.rstrip()[: -len(", user.ID)")] + ", 1, user.ID)"
            elif method in LITERAL_METHODS:
                literal = "CreateContentTypeRequest{" if method == "CreateContentType" else "ListEntriesParams{"
                if literal in line:
                    if line.rstrip().endswith("})"):
                        idx = line.rfind("})")
                        line = line[:idx] + "}, 1)" + line[idx + 2:]
                    else:
                        pending = method
                elif line.rstrip().endswith(")"):
                    # variable-arg form, e.g. CreateContentType(req)
                    line = line.rstrip()[:-1] + ", 1)"
            else:  # APPEND_METHODS, single line
                stripped = line.rstrip()
                if stripped.endswith("()"):
                    line = stripped[:-2] + "(1)"
                elif stripped.endswith(")"):
                    line = stripped[:-1] + ", 1)"
                else:
                    # call closes mid-line, e.g. `svc.DeleteX("a"); err != nil {`
                    idx = line.find(")", m.end())
                    if idx != -1:
                        line = line[:idx] + ", 1" + line[idx:]
        out.append(line)
    return "\n".join(out)

for path in FILES:
    with open(path, encoding="utf-8") as f:
        src = f.read()
    dst = process(src)
    os.makedirs(r"scripts/out", exist_ok=True)
    out_path = os.path.join(r"scripts/out", os.path.basename(path))
    if dst != src:
        with open(out_path, "w", encoding="utf-8", newline="") as f:
            f.write(dst)
        print(f"updated {out_path}")
    else:
        print(f"unchanged {path}")

#!/usr/bin/env python3
"""Export the active Section 72A schemas and event registry from Doc 10."""

import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs" / "10-contracts-mvp-roadmap.md"
SCHEMA_DIR = ROOT / "internal" / "contracts" / "v1" / "schema"
EVENTS_GO = ROOT / "internal" / "contracts" / "v1" / "events_gen.go"
SCHEMA_PATTERN = re.compile(
    r"<!-- schemas?:[\w.]+ -->\s*```json\s*\n(.*?)^```\s*$", re.M | re.S
)
DRAFT = "https://json-schema.org/draft/2020-12/schema"
PREFIX = "urn:societas:1:"


def go_name(value: str) -> str:
    return "".join(part[:1].upper() + part[1:] for part in re.split(r"[^a-zA-Z0-9]+", value) if part)


def load_schemas(text: str) -> dict[str, dict]:
    schemas: dict[str, dict] = {}
    for match in SCHEMA_PATTERN.finditer(text):
        document = json.loads(match.group(1))
        for schema in document if isinstance(document, list) else [document]:
            schema_id = schema.get("$id")
            if not schema_id or not schema_id.startswith(PREFIX):
                raise ValueError(f"missing canonical schema ID: {schema_id!r}")
            if schema_id in schemas:
                raise ValueError(f"duplicate canonical schema ID: {schema_id}")
            schema.setdefault("$schema", DRAFT)
            schemas[schema_id] = schema
    if not schemas:
        raise ValueError("no Section 72A schema markers found")
    return schemas


def load_events(text: str, schemas: dict[str, dict]) -> list[tuple[str, str, str, str]]:
    start = text.index("## 72A.7 Event Registry")
    end = text.index("## 72A.8 Payload Schemas", start)
    rows: list[tuple[str, str, str]] = []
    for line in text[start:end].splitlines():
        cells = [cell.strip() for cell in line.split("|")[1:-1]]
        if len(cells) != 3 or not cells[0].startswith("`") or not cells[1].startswith("`"):
            continue
        event_type = cells[0].strip("`")
        schema_name = cells[1].strip("`")
        rows.append((event_type, schema_name, cells[2]))
    events: list[tuple[str, str, str, str]] = []
    names: set[str] = set()
    for event_type, schema_name, route in rows:
        if event_type in names:
            raise ValueError(f"duplicate event type: {event_type}")
        names.add(event_type)
        schema_id = PREFIX + schema_name
        if schema_id not in schemas:
            raise ValueError(f"event {event_type} references missing schema {schema_id}")
        source, destination = (part.strip() for part in route.split("→", 1))
        events.append((event_type, schema_name, source.replace("`", ""), destination.replace("`", "")))
    if not events:
        raise ValueError("no events found in Section 72A.7")
    return events


def write_schemas(schemas: dict[str, dict]) -> None:
    SCHEMA_DIR.mkdir(parents=True, exist_ok=True)
    for schema_id, schema in sorted(schemas.items()):
        name = schema_id.removeprefix(PREFIX)
        path = SCHEMA_DIR / f"{name}.json"
        path.write_text(json.dumps(schema, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def write_event_registry(events: list[tuple[str, str, str, str]]) -> None:
    lines = [
        "// Code generated from docs/10-contracts-mvp-roadmap.md; DO NOT EDIT.",
        "package v1",
        "",
        "import \"fmt\"",
        "",
        "const (",
    ]
    for event_type, _, _, _ in events:
        lines.append(f"\tEvent{go_name(event_type)} EventType = \"{event_type}\"")
    lines.extend([")", "", "var eventDefinitions = [...]EventDefinition{"])
    for event_type, schema_name, source, destination in events:
        lines.append(
            f"\t{{Type: Event{go_name(event_type)}, PayloadSchemaID: \"{PREFIX}{schema_name}\", "
            f"From: {json.dumps(source)}, To: {json.dumps(destination)}}},"
        )
    lines.extend([
        "}",
        "",
        "// EventDefinitions returns the active wire event registry in source order.",
        "func EventDefinitions() []EventDefinition {",
        "\treturn append([]EventDefinition(nil), eventDefinitions[:]...)",
        "}",
        "",
        "// Definition returns the unique definition for an event type.",
        "func Definition(eventType EventType) (EventDefinition, bool) {",
        "\tfor _, definition := range eventDefinitions {",
        "\t\tif definition.Type == eventType {",
        "\t\t\treturn definition, true",
        "\t\t}",
        "\t}",
        "\treturn EventDefinition{}, false",
        "}",
        "",
        "// NewPayload allocates the schema-specific generated payload type.",
        "func NewPayload(eventType EventType) (any, error) {",
        "\tswitch eventType {",
    ])
    for event_type, schema_name, _, _ in events:
        lines.extend([
            f"\tcase Event{go_name(event_type)}:",
            f"\t\treturn &{go_name(schema_name)}{{}}, nil",
        ])
    lines.extend([
        "\tdefault:",
        "\t\treturn nil, fmt.Errorf(\"unknown event type %q\", eventType)",
        "\t}",
        "}",
        "",
    ])
    EVENTS_GO.parent.mkdir(parents=True, exist_ok=True)
    EVENTS_GO.write_text("\n".join(lines), encoding="utf-8")


def main() -> None:
    text = DOC.read_text(encoding="utf-8")
    schemas = load_schemas(text)
    events = load_events(text, schemas)
    write_schemas(schemas)
    write_event_registry(events)
    print(f"exported {len(schemas)} schemas and {len(events)} events")


if __name__ == "__main__":
    main()

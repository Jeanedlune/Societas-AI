#!/usr/bin/env python3
"""Generate Go model types from the active Section 72A schema blocks."""

from pathlib import Path
import os
import json
import shutil
import subprocess
import sys
import tempfile

import extract_schemas

ROOT = Path(__file__).resolve().parents[2]
SCHEMA_DIR = ROOT / "internal" / "contracts" / "v1" / "schema"
OUT_DIR = ROOT / "internal" / "contracts" / "v1"
GENERATOR = os.environ.get("SOCIETAS_JSONSCHEMA_GENERATOR", "go-jsonschema")


def main() -> None:
	# Regenerate the machine-readable schema files and event registry first.
	extract_schemas.main()
	schemas = extract_schemas.load_schemas(extract_schemas.DOC.read_text(encoding="utf-8"))
	with tempfile.TemporaryDirectory(prefix="societas-contract-schemas-") as temp_dir:
		temp = Path(temp_dir)
		uri_by_id = {
			schema_id: (temp / f"{schema_id.removeprefix(extract_schemas.PREFIX)}.json").as_uri()
			for schema_id in schemas
		}
		files = []
		generator = [GENERATOR] if shutil.which(GENERATOR) else ["go", "run", "github.com/atombender/go-jsonschema@v0.24.1"]
		command = generator + ["--only-models", "--package", "v1"]
		for schema_id in sorted(schemas):
			name = schema_id.removeprefix(extract_schemas.PREFIX)
			input_path = temp / f"{name}.json"
			output = OUT_DIR / f"schema_{name}.go"
			data = rewrite_refs(schemas[schema_id], schemas)
			data["$id"] = uri_by_id[schema_id]
			input_path.write_text(json.dumps(data), encoding="utf-8")
			command.extend([
				f"--schema-package={uri_by_id[schema_id]}=github.com/Jeanedlune/Societas-AI/internal/contracts/v1",
				f"--schema-output={uri_by_id[schema_id]}={output}",
				f"--schema-root-type={uri_by_id[schema_id]}={schema_root_type(name)}",
			])
			files.append(str(input_path))
		subprocess.run(command + files, cwd=ROOT, check=True)


def schema_root_type(name):
	# Envelope has a hand-written wire type so payload stays raw until dispatch.
	if name == "envelope":
		return "EnvelopeSchema"
	return extract_schemas.go_name(name)


def rewrite_refs(value, schemas, stack=()):
	if isinstance(value, dict):
		ref = value.get("$ref")
		if isinstance(ref, str) and ref.startswith(extract_schemas.PREFIX):
			base, marker, fragment = ref.partition("#")
			if base not in schemas:
				raise ValueError(f"schema reference points to unexported schema {base}")
			key = (base, fragment)
			if key in stack:
				raise ValueError(f"recursive cross-schema reference cannot be inlined: {ref}")
			target = schemas[base]
			if marker and fragment:
				for part in fragment.lstrip("/").split("/"):
					target = target[part.replace("~1", "/").replace("~0", "~")]
			return rewrite_refs(target, schemas, stack + (key,))
		return {key: rewrite_refs(child, schemas, stack) for key, child in value.items()}
	if isinstance(value, list):
		return [rewrite_refs(child, schemas, stack) for child in value]
	return value


if __name__ == "__main__":
	try:
		main()
	except subprocess.CalledProcessError as error:
		sys.exit(error.returncode)

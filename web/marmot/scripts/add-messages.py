#!/usr/bin/env python3
"""Add or update UI messages in the message catalogs under a lock, so parallel edits never
overwrite each other.

Usage: scripts/add-messages.py keys.json
where keys.json is {"key": {"en": "...", "es": "..."}, ...}. "es" is optional and should normally
be omitted: new strings go to en.json only, and es.json is left untouched unless a translation is
actually supplied. Keys starting with cloud_ go to the cloud catalogs in messages/cloud/.
"""
import collections
import fcntl
import json
import pathlib
import subprocess
import sys

root = pathlib.Path(__file__).resolve().parent.parent
pending = json.loads(pathlib.Path(sys.argv[1]).read_text())

with open(root / 'messages' / '.lock', 'w') as lock:
    fcntl.flock(lock, fcntl.LOCK_EX)
    touched = []
    for lang in ('en', 'es'):
        for subdir, wanted in (('', lambda k: not k.startswith('cloud_')), ('cloud', lambda k: k.startswith('cloud_'))):
            updates = {key: text[lang] for key, text in pending.items() if lang in text and wanted(key)}
            if not updates:
                continue
            path = root / 'messages' / subdir / f'{lang}.json'
            if not path.exists():
                sys.exit(f'{path} does not exist: cloud_ keys belong to the cloud edition, not here')
            data = json.loads(path.read_text(), object_pairs_hook=collections.OrderedDict)
            data.update(updates)
            schema = data.pop('$schema')
            out = collections.OrderedDict([('$schema', schema)])
            for key in sorted(data):
                out[key] = data[key]
            path.write_text(json.dumps(out, ensure_ascii=False, indent='\t') + '\n')
            touched.append(path)

    # Formatted while the lock is still held, so a parallel writer never reads a half-formatted file.
    for path in touched:
        subprocess.run(['npx', 'prettier', '--write', str(path)], cwd=root, check=True)

print(f'added {len(pending)} keys to {", ".join(str(p.relative_to(root / "messages")) for p in touched)}')

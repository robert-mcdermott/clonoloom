"""Independent YAML 1.2 node oracle; never construct binary floating-point values.
uv run --no-project --with ruamel.yaml==0.18.6 validation/reference/yaml_metadata.py
Compare nodes/tags with committed expected JSON consumed by native Go tests.
The optional --write regenerates these independent references, not application output.
"""
import json
import re
import sys
from pathlib import Path
from ruamel.yaml import YAML
from ruamel.yaml.nodes import MappingNode, SequenceNode, ScalarNode

yaml = YAML(typ='safe', pure=True)
yaml.version = (1, 2)
number = re.compile(r'-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?\Z')

def encode(node):
    if isinstance(node, MappingNode):
        keys = []
        fields = []
        for key, value in node.value:
            assert key.tag == 'tag:yaml.org,2002:str'
            assert key.value not in keys
            keys.append(key.value)
            fields.append(json.dumps(key.value) + ':' + encode(value))
        return '{' + ','.join(fields) + '}'
    if isinstance(node, SequenceNode):
        return '[' + ','.join(encode(value) for value in node.value) + ']'
    assert isinstance(node, ScalarNode)
    tag = node.tag.removeprefix('tag:yaml.org,2002:')
    if tag == 'null': return 'null'
    if tag == 'bool': return node.value.lower()
    if tag in ('int', 'float'):
        assert number.fullmatch(node.value), node.value
        return node.value
    assert tag == 'str', tag
    return json.dumps(node.value, ensure_ascii=True)

root = Path(__file__).resolve().parents[2] / 'testdata/metadata/yaml'
for source in sorted(root.glob('*.yaml')):
    expected = encode(yaml.compose(source.read_text())) + '\n'
    path = source.with_suffix('.json')
    if '--write' in sys.argv: path.write_text(expected)
    else: assert path.read_text() == expected, source
    print(f'{source.name}: exact YAML tags, strings and numeric tokens checked')

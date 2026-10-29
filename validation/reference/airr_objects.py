"""Independent exact JSON/YAML object oracle and AIRR receptor hash check.
uv run --with ruamel.yaml==0.18.6 validation/reference/airr_objects.py /tmp/objects-native.json
--write creates the declared simulated fixture and independent YAML expected JSON.
No application code imported; this is format/link evidence, not biological validation.
"""
import hashlib
import json
import sys
from decimal import Decimal
from pathlib import Path
from ruamel.yaml import YAML
from ruamel.yaml.scalarstring import DoubleQuotedScalarString
from yaml_metadata import encode

root = Path(__file__).resolve().parents[2] / 'testdata/airr-objects'
yaml = YAML()
yaml.width = 4096
yaml.version = (1, 2)
if '--write' in sys.argv:
    beta = 'DTQYFCAQGTVSLSVIEQTPRHLVRIGGTSVTVTCEGQSLHETMYWYRQDPGQGLRLIYYSASEGTTDKGEVPNGYNVSRLNKREFSLRLESAAPSQTSVYFCASSLGQETQYFGPGTRLLVL'
    alpha = 'AQSVTQLGSHVSVSEGALVLLRCNYSSSVPPYLFWYVQYPNQGLQLLLKYLSGSTLVESINGFEAEFKKSETSFHLTKPSAHMSDAAEYFCAVRDSNYQLIWGAGTKLIIKP'
    doc = {'Info': {'version':'2.0','title':'Simulated AIRR objects; no biological claim'},
      'Repertoire':[{'repertoire_id':'R001','repertoire_type':'simulated','data_processing':[{'data_processing_id':'p1'}]}],
      'Clone':[{'clone_id':'source-family-1','repertoire_id':'R001','repertoire_group_id':None,'clone_class':'cell','data_processing_id':'p1','clone_count':2,
        'nodes':[{'node_id':'n1','repertoire_id':'R001','cell_id':'C001','node_type':'simulated'},{'node_id':'n2','repertoire_id':'R001','cell_id':'C002','node_type':'simulated'}],
        'extension':{'integer':9007199254740993,'precise':'DECIMAL_TOKEN','flag':False,'missing':None}}],
      'Cell':[{'cell_id':'C001','repertoire_id':'R001','data_processing_id':'p1','virtual_pairing':False,'cell_type':'simulated','receptors':['sim-receptor']},
              {'cell_id':'C002','repertoire_id':'R001','data_processing_id':'not-loaded','virtual_pairing':None,'cell_type':'simulated','receptors':['outside-opening']}],
      'Receptor':[{'receptor_id':'sim-receptor','receptor_type':'TCR','receptor_variable_domain_1_locus':'TRB','receptor_variable_domain_2_locus':'TRA','receptor_variable_domain_1_aa':beta,'receptor_variable_domain_2_aa':alpha,'receptor_hash':hashlib.sha256((beta+alpha).encode()).hexdigest(),'extension':{'simulated':True,'note':'Constructed domain-like sequences; not an experimentally observed receptor or functional prediction'}}],
      'Node':[{'node_id':'outside-node','repertoire_id':None,'sequence_id':'unobserved-ref','node_type':'inferred'}],
      'lab_extension':{'simulated':True,'note':'All sequences, cells, membership and metadata are artificial. No activity, ancestry or pairing claim.'}}
    root.joinpath('study.json').write_text(json.dumps(doc,indent=2).replace('"DECIMAL_TOKEN"','1.234567890123456789')+'\n')
    # YAML independent writer; use an exact integer/decimal lexical test separately.
    doc['Receptor'][0]['receptor_hash'] = DoubleQuotedScalarString(doc['Receptor'][0]['receptor_hash'])
    with root.joinpath('study.yaml').open('w') as out: yaml.dump(doc,out)
    p=root/'study.yaml';p.write_text(p.read_text().replace('%YAML 1.2\n','').replace('DECIMAL_TOKEN','1.234567890123456789'))
    root.joinpath('study.expected.json').write_text(encode(yaml.compose(p.read_text()))+'\n')
else:
    source=json.loads((root/'study.json').read_text(),parse_float=Decimal)
    expected=json.loads((root/'study.expected.json').read_text(),parse_float=Decimal)
    assert source==expected
    assert json.loads(encode(yaml.compose((root/'study.yaml').read_text())),parse_float=Decimal)==source
    native=json.loads(Path(sys.argv[1]).read_text())
    for record in native:
        value=source
        for part in record['path'].split('/')[1:]: value=value[int(part)] if isinstance(value,list) else value[part]
        assert json.loads(record['rawJSON'],parse_float=Decimal)==value,record['path']
        if record['kind']=='Receptor':
            actual=hashlib.sha256((value['receptor_variable_domain_1_aa'].upper()+value['receptor_variable_domain_2_aa'].upper()).encode()).hexdigest()
            assert actual==value['receptor_hash']
    assert len(native)==7
    print('7 original objects, independent JSON/YAML exact values and receptor hash verified')

"""Installed distribution smoke check; run without PYTHONPATH and with uv --with WHEEL."""
from decimal import Decimal
from importlib.metadata import version
from pathlib import Path
import clonoloom

assert version('clonoloom') == clonoloom.__version__
assert clonoloom.API_VERSION == 2 and clonoloom.POLICY_VERSION == 1
assert 'site-packages' in str(Path(clonoloom.__file__)), 'Imported checkout, not installed package'
result = clonoloom.Result('fixture', '', {'resultJSON':'{"rows":[{"n":9007199254740993,"x":0.123456789012345678901}]}'} )
assert result.records()[0] == {'n':9007199254740993,'x':Decimal('0.123456789012345678901')}
assert callable(clonoloom.ClonoLoom.query_clonotypes)
print('Installed package passed: version, generated methods and exact JSON values.')

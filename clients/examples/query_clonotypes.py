"""uv run --with /path/to/clonoloom/clients/python clients/examples/query_clonotypes.py

Start the development host with --remote-control --allow compute,files and open a dataset first.
This example selects rows; it does not infer specificity or renormalize abundance.
"""
import argparse
from decimal import Decimal
import clonoloom

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--data-dir', help='The running host library; otherwise use normal discovery')
args = parser.parse_args()
client = clonoloom.connect(data_dir=args.data_dir, client='Python repertoire query')
result = client.query_clonotypes(where={'field': 'total', 'op': 'gte', 'value': Decimal('2')}, limit=10)
evidence = result.native()
print(result.message)
print('Full table:', evidence['count'], 'matching:', evidence['matched'], 'definition:', evidence['definitionId'])
for row in result.records():
    print(row['id'], row['total'], row['frequency'])
# Original scientific JSON remains result.data['resultJSON']; parsed frequencies are Decimal.
# An optional result.frame() keeps object columns. Float conversion would be an explicit choice.

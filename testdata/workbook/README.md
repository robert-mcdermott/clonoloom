# Artificial workbook precision controls

`precision.json` is an authored format stress fixture under ClonoLoom's Apache-2.0 license.
It contains no participant or provider data and is not a plausible biological study.
It checks exact integers/decimals, negative zero, magnitude limits, strings that resemble IDs,
dates and formulas, booleans, zero, null, absent fields, empty containers/strings, JSON-pointer
escaping, Unicode, control characters and literal SpreadsheetML escape-looking text.

Generate the workbook through `TestWorkbookExactEvidence`, with `CLONOLOOM_WORKBOOK_OUT`
pointing to a disposable directory. `validation/reference/workbook.py` independently reads it.

# csv-autopsy

[![ci](https://github.com/johnmikel/csv-autopsy/actions/workflows/ci.yml/badge.svg)](https://github.com/johnmikel/csv-autopsy/actions/workflows/ci.yml)

**What did the spreadsheet do to your file?**

Open a CSV in Excel or Google Sheets, save it, and some of it quietly changes:

| You had | After a spreadsheet | Can it be undone? |
|---|---|---|
| `07700900939` | `7700900939` | Yes: the 0 goes back |
| `2026010500077080` | `2.02601E+15` | No: the digits are gone |
| `05/01/2026` (5 January) | `5/1/2026`, or `2026-05-01` (1 May) | Yes |
| `2026-01-11` | `46033` | Yes |
| `Peñaflor` | `PeÃ±aflor` | Yes |
| `2-3` (a size) | `02-Mar` | No: it was `2-3` or `3-2` |

csv-autopsy reads a file that has been through a spreadsheet, says in plain
English what was damaged and where, and repairs what can be repaired.

**[Try it in your browser](https://johnmikel.github.io/csv-autopsy/)**. It opens
on a sample file, and your own files never leave the page: the checks run in
the browser, compiled from Go to WebAssembly.

## Use it

```sh
go install github.com/johnmikel/csv-autopsy@latest

csv-autopsy orders.csv                             # what was damaged
csv-autopsy -fix orders-repaired.csv orders.csv    # and a repaired copy
csv-autopsy -json orders.csv                       # the findings as JSON
```

Here it is on [`testdata/orders-damaged.csv`](testdata/orders-damaged.csv), an
online shop's orders after one trip through a spreadsheet:

```
orders-damaged.csv: 40 rows, 9 columns

Found 7 problems. 5 of them can be repaired with -fix.

order_id: 40 order numbers were turned into scientific notation, like 2.02601E+15 [cannot be undone]
    2.02601E+15 keeps only the first 6 digits of the real number. The rest are
    gone.
    Rows 2, 3, 4, 5, 6 and 35 more.
    Get these again from the system that made the file. Next time, import this
    column as text.

phone: 32 phone numbers lost their leading 0 [can be repaired]
    7700900939 should be 07700900939. A spreadsheet took it for a number and
    dropped the 0 at the front.
    Rows 2, 3, 4, 5, 7 and 27 more.
    Repair puts the 0 back. Next time, import this column as text.

ordered_on: 16 dates were rewritten by a spreadsheet that reads dates month first [can be repaired]
    Most dates here are day first and written like 15/01/2026. These look like
    5/1/2026 instead. They still mean the right date when read day first, but
    any program that reads them month first gets the day and month the wrong
    way round.
    Rows 2, 3, 4, 11, 12 and 11 more.
    Repair writes every date in this column as YYYY-MM-DD, which no program
    can misread.
…
```

It exits with status 1 when it finds damage, so it can stop a data pipeline
before a damaged file gets loaded.

## What it looks for

| Damage | How it knows | Repairable |
|---|---|---|
| Numbers that lost their leading 0 | Other values in the column kept theirs, or the column holds a format that always starts with 0: UK phone numbers, Philippine mobiles (09…), US ZIP codes | Yes |
| Long numbers in scientific notation | `4.11111E+15` in a column of IDs or phone numbers | No |
| Long numbers cut to 15 digits | Every number over 15 digits in the column ends in zeros | No |
| Dates turned into numbers | Numbers such as `46033` in a date column, or among dates | Yes |
| Dates rewritten by a month-first reader | The dates that can only be day first (`13/04/2026`) are written one way, and the ambiguous ones (`3/4/2026`) another | Yes |
| Dates swapped by a month-first reader | Day-first dates, plus `YYYY-MM-DD` dates whose day is never above 12 | Yes |
| Day-first and month-first dates mixed | Both `13/04/2026` and `04/13/2026` in one column | No |
| Text turned into dates | `02-Mar` among sizes, ranges or codes | No |
| Scrambled letters | Text that becomes valid UTF-8 again when read back as the Windows-1252 it was mistaken for, even twice over | Yes |
| A file saved as Windows-1252 | The bytes are not valid UTF-8 | Yes |
| A file cut off at a spreadsheet's limit | Exactly 65,536 or 1,048,576 rows, or 256 or 16,384 columns | No |
| Text cut off at a cell's limit | A value exactly 32,767 characters long | No |

Repaired dates are written as `YYYY-MM-DD`, which no program can read the
wrong way round.

## Why

Spreadsheets have done this for decades. In 2020 the committee that names
human genes renamed 27 of them, `SEPT1` to `SEPTIN1` and `MARCH1` to
`MARCHF1` among them, because Excel kept turning them into dates. Tools such as
[Escape Excel](https://github.com/biodatalab/escape_excel) and
[csvmedic](https://pypi.org/project/csvmedic/) help stop the damage before a
file is opened. csv-autopsy is for afterwards: when the file you were sent has
already been through a spreadsheet, and you need to know what you can trust.

## Development

```sh
make test     # go vet and go test
make page     # the web page in dist/, with the checker compiled to WebAssembly
make sample   # regenerate testdata from testdata/generate.go
```

The tests check every kind of damage on small cases, run the whole sample
through and compare the repair with the original file, and make sure the
original raises no false alarms.

## License

[MIT](LICENSE)

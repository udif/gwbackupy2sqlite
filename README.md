gwbackupy2sqlite
================

What is gwbackupy2sqlite?
-------------------------

gwbackupy2sqlite, as its name implies converts data from gwbackupy to sqlite.
[gwbackupy](https://github.com/smartondev/gwbackupy) is a utility that can backup and restore a gmail account into your local disk.
While this utility does perform a backup, it is not easy to browse the backup files.  
What [gwbackupy2sqlite](https://github.com/udif/gwbackupy2sqlite) does, it to extract metadata from this backup and create an sqlite database out of it.
This metadata is stored inside an [SQLite](https://www.sqlite.org/) database. Once the data is in an SQLite database, it is easy to browse it through tools such as [datasette](https://datasette.io/).

Requirements
------------
* Python3
* [gwbackupy](https://github.com/smartondev/gwbackupy)
* [datasette](https://datasette.io/)

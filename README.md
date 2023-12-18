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
* Python3 (I'm using 3.11 at the moment, not sure which version is the minimum required)
* [gwbackupy](https://github.com/smartondev/gwbackupy)
* [datasette](https://datasette.io/)

Usage:
------
1. Prepare a gmail backup using [gwbackupy](https://github.com/smartondev/gwbackupy) by following its instructions. Let's say the backup destination is in the default directory, named 'data'
2. Prepare your own copy of `sample_email_addr_table.sql` and add your real email addresses and/or personal domains.
3. Run the following command: `gwbackupy2sqlite --dir data --db your_db_name.sqlite -f sample_email_addr_table.sql`
3. Run `datasette your_db_name.sqlite` and open `http://127.0.0.1:8001/` On your local browser.

Why is this useful?
-------------------
By analyzing the backup through datasette it is possible to do complex SQL queries over your email database metadata, for example:
1. Sort your emails in groups by the sender, sorting groups by their collective size from the largest to smallest.

TODO
----
This is a quick & dirty project. It is not very fast, but by selectively scanning only new email folders the delay is not long.
(gwbackupy stores emails in a separate folder for every day, all inside a year folder). It is also my first projetg using SQL.
1. Parallelize it as much as possible. How to do this in python is yet to be seen.
2. Add more features for directly reading the emails from datasette.
3. Rewrite it in a faster language such as Go??

License
-------
The license chosen for this program is the same BSD 3-Clause license used by gwbackupy, for this reason exactly.


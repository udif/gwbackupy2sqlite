--
-- Sample email address database file.
-- Replace the sample email addresses and domains here with your real email address and/or domain.
-- Only emails matching this records will be shown when browsing the my_mails view in datasette
-- To use this file, include it with the gwbackupy2sqlite run using the -f flag
--
insert or ignore into my_email_addresses (addr) values ("sample@gmail.com"), ('%@sample.com');

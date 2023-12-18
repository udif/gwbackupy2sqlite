-- This is the main email table
-- each record is a single email
CREATE TABLE if not exists emails (
  id BIGINT PRIMARY KEY NOT NULL,
  threadId TEXT KEY,
  subject_e TEXT,
  snippet TEXT,
  historyId INT,
  sizeEstimate INT,
  internalDate BIGINT,
  date_e INTEGER
);

-- This tables holds the label names and their ID
CREATE TABLE if not exists labels (
  id INTEGER PRIMARY KEY,
  lname TEXT UNIQUE
);

-- This table links labels and emails
-- Each entry links a specific label with a specific email by their IDs
CREATE TABLE if not exists email_label (
  email_id BIGINT,
  label_id INTEGER,
  FOREIGN KEY (email_id) REFERENCES emails (id),
  FOREIGN KEY (label_id) REFERENCES labels (id),
  PRIMARY KEY (email_id, label_id)
);

-- This table has a record for each attachment.
-- Each attachment has a name, size, an ID (key) and is linked to the email ID it is found within
-- There can be multiple attachments pointing to the same email.
CREATE TABLE if not exists attachments (
  id INTEGER PRIMARY KEY,
  email_id BIGINT,
  att_name TEXT,
  att_size INT,
  FOREIGN KEY (email_id) REFERENCES emails (id)
);

-- This tables stores unique email addresses, indexed by an ID
CREATE TABLE if not exists email_addresses (
  id INTEGER PRIMARY KEY,
  addr_e TEXT KEY UNIQUE 
);

-- This tables stored unique names, indexed by an ID
CREATE TABLE if not exists email_names (
  id INTEGER PRIMARY KEY,
  name_e TEXT KEY UNIQUE 
);

-- This tables links an email with a specific address and name representing a single sender field (From:)
-- There can be multiple records pointing to the same email
CREATE TABLE if not exists email_from (
  id INTEGER PRIMARY KEY,
  email_id BIGINT,
  email_name INT,
  email_address INT,
  FOREIGN KEY (email_id) REFERENCES emails (id),
  FOREIGN KEY (email_name) REFERENCES email_names (id),
  FOREIGN KEY (email_address) REFERENCES email_addresses (id)
);

-- This tables links an email with a specific address and name representing a single sender field (Reply-To:)
-- There can be multiple records pointing to the same email
CREATE TABLE if not exists email_reply_to (
  id INTEGER PRIMARY KEY,
  email_id BIGINT,
  email_name INT,
  email_address INT,
  FOREIGN KEY (email_id) REFERENCES emails (id),
  FOREIGN KEY (email_name) REFERENCES email_names (id),
  FOREIGN KEY (email_address) REFERENCES email_addresses (id)
);

-- This tables links an email with a specific address and name representing a single recipient field (To: or CC:, etc.)
-- There can be multiple records pointing to the same email
CREATE TABLE if not exists email_to (
  id INTEGER PRIMARY KEY,
  email_id BIGINT,
  email_name INT,
  email_address INT,
  FOREIGN KEY (email_id) REFERENCES emails (id),
  FOREIGN KEY (email_name) REFERENCES email_names (id),
  FOREIGN KEY (email_address) REFERENCES email_addresses (id)
);

-- This tables links an email with a specific address and name representing a single sender field (Sender:)
-- Since there can only be a single Sender: field, email_id is unique
CREATE TABLE if not exists email_sender (
  id INTEGER PRIMARY KEY,
  email_id BIGINT UNIQUE,
  email_name INT,
  email_address INT,
  FOREIGN KEY (email_id) REFERENCES emails (id),
  FOREIGN KEY (email_name) REFERENCES email_names (id),
  FOREIGN KEY (email_address) REFERENCES email_addresses (id)
);

-- This table holds 
CREATE TABLE if not exists my_email_addresses (
  addr TEXT UNIQUE
);

create view full_mails as select
  e.id AS email_id,
  fn.name_e as fname,
  fa.addr_e as faddr,
  tn.name_e as tname,
  ta.addr_e as taddr,
  e.subject_e,
  strftime('%d/%m/%Y, %H:%M:%S', datetime(e.date_e, 'unixepoch'), "localtime") as date_txt
from
  emails e,
  email_from f,
  email_to t,
  --email_sender s,
  --email_reply_to r,
  email_addresses fa,
  email_names fn,
  email_addresses ta,
  email_names tn
where
  e.id = t.email_id
  and e.id = f.email_id
  and t.email_name = tn.id
  and t.email_address = ta.id
  and f.email_name = fn.id
  and f.email_address = fa.id;

-- A specific view to filter only mails intended to me
create view my_mails as SELECT 
    e.id AS email_id,
    e.subject_e AS email_subject,
    fn.name_e || ' <' || fa.addr_e || '>' AS from_n_a,
    tn.name_e || ' <' || ta.addr_e || '>' AS to_n_a,
    strftime('%d/%m/%Y, %H:%M:%S', datetime(e.date_e, 'unixepoch'), "localtime") as date_txt,
    e.sizeEstimate as size_estimate
FROM 
    emails e
LEFT JOIN 
    email_sender es ON e.id = es.email_id
JOIN 
    email_from ef ON es.email_id is null and e.id = ef.email_id
JOIN 
    email_to et ON e.id = et.email_id
JOIN 
    email_names fn ON ef.email_name = fn.id or es.email_name = fn.id
JOIN 
    email_addresses fa ON ef.email_address = fa.id or es.email_address = fa.id
JOIN 
    email_names tn ON et.email_name = tn.id
JOIN 
    email_addresses ta ON et.email_address = ta.id
JOIN
    my_email_addresses mea ON ta.addr_e LIKE mea.addr;

create view from_by_size as select
  from_n_a,
  sum(size_estimate) as total_size,
  count(8) as count
from
  my_mails
group by
  from_n_a
order by
  total_size desc
limit
  101;

-- In the context of database schemas, the three fields in a semantic version number (MAJOR.MINOR.PATCH) can have the following meanings:
-- Major: This field is incremented when there are incompatible changes that would break existing systems. For example, removing a table or changing the data type of a column.
-- Minor: This field is incremented when functionality is added in a backwards-compatible manner. For example, adding a new table or adding a new column to an existing table.
-- Patch: This field is incremented when backwards-compatible bug fixes are made. For example, fixing a default value or correcting a data type.
CREATE TABLE if not exists schema_version (
  major INT,
  minor INT,
  patch INT
);

insert into schema_version (major, minor, patch) values (1, 0, 0);

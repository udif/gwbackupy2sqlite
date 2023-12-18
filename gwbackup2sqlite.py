#
# gwbackupy2sqlite.py
#
# convert a gwbackupy database to sqlite for queries
#
# Copyright © 2023 by Udi Finkelstein
#
# Released under the BSD 3-clause license. See LICENSE file.
#
import os
import sys
import sqlite_utils
import datetime
import email.utils
from email.header import decode_header
import pathlib
import argparse
import email
import gzip
import json
from datetime import datetime

labels = {}
schema_version = (1,0,0)

class gm_json:
    def __init__(self, db):
        global labels
        self.db = db
        if db:
            self.labels_db = db["labels"]
            for row in db.query("select * from labels"):
                labels[row["lname"]] = row["id"]
        else:
            labels = {}

    def handle_json(self, id, data):
        for key in data:
            if key == 'labelIds':
                for l in data[key]:
                    if l not in labels:
                        if db:
                            self.labels_db.insert({"lname": l})
                            labels[l] = self.labels_db.last_rowid
                        else:
                            labels[l] = len(labels)
            elif key == 'id':
                if id != int(data[key], 16):
                    print(f"Illegal id! filename_id={hex(id)} json_id={data[key]}")
                    sys.exit(1)
                self.db["emails"].upsert({key: id}, pk="id")
            elif key == 'threadId':
                self.db["emails"].upsert({"id": id, key: int(data[key], 16)}, pk="id")
            else:
                self.db["emails"].upsert({"id": id, key: data[key]}, pk="id")

# https://stackoverflow.com/questions/7331351/python-email-header-decoding-utf-8
def encoded_words_to_text(encoded_words):
    try:
        dh = decode_header(encoded_words)
        return ''.join([ str(t[0], 'utf-8') if t[1] == 'unknown-8bit'
                         else str(t[0], 'iso-8859-8') if t[1] == 'iso-8859-8-i'
                         else str(t[0], t[1]) if t[1] is not None
                         else t[0] if isinstance(t[0], str)
                         else str(t[0], 'utf-8')
                         for t in dh ])
    except:
        print (f"exception: dh:{dh} encoded_words:{encoded_words}")
        sys.exit(1)

def insert_and_return_rowid(db, column, value):
    db.insert({column : value}, ignore=True)
    rows = list(db.rows_where(f"{column} = ?", [value]))
    if len(rows) > 1:
        print("More than 1 line on a unique value", column, value)
        sys.exit(1)
    return rows[0]["id"]

def get_attachment_list_and_size(db, id, email_string):
    fields = {}
    fields["from"] = set()
    fields["to"] = set()
    fields["reply-to"] = set()
    fields["sender"] = set()
    fields["resent-cc"] = fields["resent-to"] = fields["cc"] = fields["to"]
    fs = fields["sender"]
    ff = fields["from"]
    ft = fields["to"]
    fr = fields["reply-to"]
    emails = db["emails"]
    email_names = db["email_names"]
    email_addresses = db["email_addresses"]
    email_from = db["email_from"]
    email_to = db["email_to"]
    email_reply_to = db["email_reply_to"]
    email_sender = db["email_sender"]
    attachments = db["attachments"]
    emails.upsert({"id": id}, pk="id")

    # Assume email_string is the MIME email stored in a string
    msg = email.message_from_bytes(email_string)
    subject_e = ""
    date_e = 0
    for i in msg.items():
        j = i[0].lower()
        if j in fields:
            fields[j].add(i[1])
        elif j == "subject":
            subject_e = encoded_words_to_text(i[1])
        elif j == "date":
            dt = email.utils.parsedate_to_datetime(i[1])
            date_e = round(datetime.timestamp(dt))
    if len(fs) > 1:
        print(f"More than 1 Sender: {fs}")
        sys.exit(1)
    if len(fs) == 0 and len(fields["from"]) > 1:
        print(f"More than 1 From: {ff} and no Sender:")
        sys.exit(1)
    emails.upsert({"subject_e": subject_e, "date_e": date_e, "id":id}, pk="id")
    fr = email.utils.getaddresses(list(ff))
    to = email.utils.getaddresses(list(ft))
    reply_to = email.utils.getaddresses(list(fr))
    sender = email.utils.getaddresses(list(fs))
    #print(f"from:{fr} to:{to} sender:{sender} reply-to:{reply_to}")
    for (d,field) in ((email_sender, sender), (email_from, fr), (email_to, to), (email_reply_to, reply_to)):
        for i in field:
            lpka=insert_and_return_rowid(email_addresses, "addr_e", i[1].lower())
            #print("Addr:", i[1].lower(), lpka)
            name = encoded_words_to_text(i[0])
            lpkn = insert_and_return_rowid(email_names, "name_e", name.lower())
            #print("Name:", name.lower(), lpkn)
            d.insert({"email_id": id, "email_name":lpkn, "email_address":lpka}, ignore=True)
    if not msg.is_multipart():
        return None
    # Iterate over the parts of the email
    for part in msg.walk():
        # Check if the part is an attachment
        if part.get_content_maintype() == 'multipart':
            continue
        if part.get('Content-Disposition') is None:
            continue

        # Get the filename and size of the attachment
        t = part.get_content_type()
        if t in ['message/rfc822', 'text/plain']:
            continue
        filename = part.get_filename()
        if (filename):
            size = len(part.get_payload(decode=True))
            attachments.insert({"att_name": filename, "att_size":size, "email_id": id})

# return db handle, create if doesn't exist
def open_database(db_name):
    global schema_version
    db = sqlite_utils.Database(db_name)
    rows = list(db.query("SELECT name FROM sqlite_master WHERE type='table' AND name='schema_version';"))
    if len(list(rows)) > 0:
        rows = list(db.query("SELECT * FROM schema_version;"))
        current_schema_version = (rows[0]['major'], rows[0]['minor'], rows[0]['patch'])
        print(f"Version: {rows[0]['major']}.{rows[0]['minor']}.{rows[0]['patch']}")
    else:
        print("No DB found, creating a new one.")
        current_schema_version = schema_version
        with open(pathlib.Path(__file__).parent.resolve() / "tables.sql") as f:
            db.executescript(f.read())
    if schema_version > current_schema_version:
        update_db(db, current_schema_version)
    return db

# Update DB if schema changes
# current_schema_version - current schema of DB file
def update_db(db, current_schema_version):
    pass
    
def get_file_list(dir, db=None):
    path = pathlib.Path(dir)
    j = gm_json(db)
    idx = 0
    for n in path.rglob("*"):
        idx = idx + 1
        if idx % 100 == 0:
            print(f"Index:{idx}", end='\r')
        ns = str(n)
        if not os.path.isfile(n):
            continue
        id = os.path.basename(n).split('.')[0]
        try:
            id = int(id, 16)
        except:
            continue
        if not ns.endswith(".json"):
            if not ns.endswith(".gz"):
                print("Unknown file type found:", os.path.basename(n))
                sys.exit(1)
            with gzip.open(ns, 'rb') as f:
                get_attachment_list_and_size(db, id, f.read())
                continue
        with open(ns) as json_file:
            data = json.load(json_file)
            j.handle_json(id, data)

#
# THis is debugging code to test just the subject line decoding without all the overheard
#
def test_email_decode(dir):
    idx = 0
    path = pathlib.Path(dir)
    for n in path.rglob("*"):
        idx = idx + 1
        if idx % 100 == 0:
            print(f"Index:{idx}",end='\r')
        ns = str(n)
        if not os.path.isfile(n):
            continue
        id = os.path.basename(n).split('.')[0]
        try:
            idd = int(id, 16)
        except:
            continue
        if not ns.endswith(".gz"):
            continue
        with gzip.open(ns, 'rb') as f:
            msg = email.message_from_bytes(f.read())
            for i in msg.items():
                j = i[0].lower()
                if j == "subject":
                    try:
                        encoded_words_to_text(i[1])
                        break
                    except:
                        print(n)
                    break


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description="Export gwbackupy data directory into sqlite")
    #parser.add_argument("-h", "--help", action="help", help="Show this help message and exit.")
    parser.add_argument("--test-subject", default=False, help="Only decode subject line to test it", action="store_true")
    parser.add_argument("--db", type=str, help="sqlite DB name", default=None)
    parser.add_argument("-f", "--sql-file", type=str, help="extra SQL command file to run", default=None)
    parser.add_argument("--dir", type=str, help="gwbackupy directory name", default=None)
    parser.add_argument('rest', nargs=argparse.REMAINDER)
    args = parser.parse_args()
    if not args.db or not args.dir:
        print("Please supply sqlite DB name and gwbackupy directory name")
        sys.exit(1)
    if args.test_subject:
        test_email_decode(args.dir)
        sys.exit(0)
    db = open_database(args.db)
    if args.sql_file:
        with open(pathlib.Path(__file__).parent.resolve() / args.sql_file) as f:
            db.executescript(f.read())
    l = get_file_list(args.dir, db)
    sys.exit(0)

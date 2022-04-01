#!/usr/bin/env python3
from __future__ import print_function

import os.path
import logging
import sys
import tempfile
import os

from google.auth.transport.requests import Request
from google.oauth2.credentials import Credentials
from google_auth_oauthlib.flow import InstalledAppFlow
from googleapiclient.discovery import build
from googleapiclient.errors import HttpError

# If modifying these scopes, delete the file token.json.
SCOPES = ['https://www.googleapis.com/auth/gmail.modify']

root = logging.getLogger()

def get_next_page(service, next_page_token, callback):
    results = service.users().messages().list(userId='me', q='is:unread older_than:1d', pageToken=next_page_token).execute()
    messages = results.get('messages', [])
    callback(service, messages)

    next_page_token = results.get('nextPageToken', None)
    if next_page_token:
        get_next_page(service, next_page_token, callback)

def archive_messages(service, messages):    
    ids = [m['id'] for m in messages]
    root.info(f'Archiving {len(ids)} messages')
    service.users().messages().batchModify(userId='me', body={'ids': ids, 'removeLabelIds': ['INBOX','UNREAD']}).execute()

def configure_logging(logger):
    logger.setLevel(logging.INFO)
    handler = logging.StreamHandler(sys.stdout)
    handler.setLevel(logging.DEBUG)
    formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
    handler.setFormatter(formatter)
    logger.addHandler(handler)

def main():
    configure_logging(root)

    """Shows basic usage of the Gmail API.
    Lists the user's Gmail labels.
    """

    root.info('Starting purge...')
    creds = None

    # read token from environment and store in temporary file
    token = os.environ['GOOGLE_TOKEN']

    if token:
        with tempfile.NamedTemporaryFile() as f:
            f.write(token.encode('utf-8'))
            f.flush()
            creds = Credentials.from_authorized_user_file(f.name)
    else:
        flow = InstalledAppFlow.from_client_secrets_file(
            'credentials.json', SCOPES)
        creds = flow.run_local_server(port=0)

    try:
        service = build('gmail', 'v1', credentials=creds)
    except HttpError as error:
        # TODO(developer) - Handle errors from gmail API.
        print(f'An error occurred: {error}')
    
    root.info('Getting unread messages...')
    get_next_page(service, None, archive_messages)

if __name__ == '__main__':
    main()
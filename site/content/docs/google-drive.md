---
title: Google Drive
description: Create a service account, share a folder with it, and point each target at its folder.
---

DBEaseBackup uploads to Google Drive as a service account. The service account has its own Drive, which you cannot browse, so share a folder from your Drive with it and set that folder per target.

## 1. Create the service account

1. In the [Google Cloud console](https://console.cloud.google.com/), select or create a project.
2. Enable the **Google Drive API** under *APIs & Services → Library*.
3. Under *IAM & Admin → Service Accounts*, create a service account. It needs no project roles.
4. Open it, go to *Keys → Add key → Create new key*, choose **JSON** and download the file.

Treat the JSON key like a password. See [Security](/docs/security/).

## 2. Share a folder

1. In Google Drive, create a folder per target, for example `backups/app-db`.
2. Share it with the service account's `client_email` (from the JSON key) as **Editor**.
3. Copy the folder ID from its URL: `https://drive.google.com/drive/folders/<FOLDER_ID>`.

One folder per target keeps [retention](/docs/retention/) listings small and separate.

## 3. Configure

```bash title=".env"
STORAGE_TYPE=google-drive
GOOGLE_DRIVE_KEY_FILE=/config/service-account-key.json
```

```yaml title="targets.yaml"
targets:
  - name: app-db
    # ...connection and schedule
    storage:
      folder_id: "1AbCdEfGhIjKlMnOpQrStUvWxYz"
```

Mount the key read-only and make sure the container user (uid 100) can read it:

```bash
docker run ... \
  -v "$(pwd)/service-account-key.json:/config/service-account-key.json:ro" \
  glennprays/dbeasebackup:latest
```

In [legacy mode](/docs/legacy-single-db/), set `GOOGLE_DRIVE_FOLDER_ID` instead of `storage.folder_id`.

## How uploads work

- Files are sent with a resumable upload in 8 MiB chunks, so memory use stays flat regardless of backup size.
- `/health` lists one file to prove the key works and the API is reachable.
- Verification and cleanup find files **by name**. Download for [verification](/docs/verification/) searches every file the service account can see and uses the first match; names include the target and a second-precision timestamp, so collisions do not happen in practice.

## Storage quota

Files uploaded by a service account into a folder in your **My Drive** are owned by the service account and count against its own quota. New service accounts may have no usable Drive storage, in which case uploads fail with `storageQuotaExceeded`. If you see that error, use a folder in a **shared drive** (Google Workspace), where files count against the shared drive, or switch to [S3](/docs/s3/).

> [!NOTE]
> Uploads do not set `supportsAllDrives`. If your shared-drive folder is rejected with `File not found`, that is the cause; use S3 until shared drives are supported.

## Limits

- Listing for cleanup reads one page (100 files) per target. Keep each target's folder below that.
- Deletes are permanent and skip the Drive trash.

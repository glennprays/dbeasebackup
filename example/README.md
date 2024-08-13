<p align="center">
  <img src="../img/logo.png" alt="Logo" width="200"/>

  <h1 align="center">DBEaseBackup</h1>
</p>

## Get Started
### Google Service Account
To utilize Google Drive for storage, you need to set up a _Google Service Account Key_ (JSON version). You can create and download this key from the [Google Cloud Console](https://console.cloud.google.com/). Once you have downloaded the key, rename the file to `service-account-key.json`. This file will then be mounted into the Docker container.

### Google Drive Folder
Obtain the id of the Google Drive folder and configure it for the Google Service Account.
1. Create a target folder.
2. Add the `client_email` (found in the `service-account-key.json file`) as an editor to the Google Drive folder.
3. Copy the `id` of the folder from its URL, which follows the format `https://drive.google.com/drive/folders/<id>`.

### Enviroment Variable
Copy the file `.env.example` and populate each variable with the appropriate configuration values. Specifically, configure the `CRON_SCHEDULE=` variable with the appropriate cron expression to set the backup schedule.

### Running DBEaseBackup
After configure all steps above, you can run it using docker compose
```
docker compose up -d
```
